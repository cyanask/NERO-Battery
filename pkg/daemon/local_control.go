// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charlie0129/batt/pkg/compatibility"
	"github.com/charlie0129/batt/pkg/config"
	"github.com/charlie0129/batt/pkg/localipc"
	"github.com/charlie0129/batt/pkg/localxpc"
	"github.com/charlie0129/batt/pkg/powerinfo"
	"github.com/charlie0129/batt/pkg/smc"
)

const LocalStateDir = "/Library/Application Support/NERO Battery"

type localStatusView struct {
	Observation powerinfo.Observation `json:"observation"`
	Limit       int                   `json:"limit"`
	Lower       int                   `json:"lower_limit"`
	Target      int                   `json:"charge_target"`
	Until       time.Time             `json:"discharge_until"`
	Charging    bool                  `json:"charging_enabled"`
	Adapter     bool                  `json:"adapter_enabled"`
	Plugged     bool                  `json:"plugged_in"`
	Network     localipc.SocketCounts `json:"helper_network"`
}

func (d *Daemon) localStatus() (localStatusView, error) {
	o, err := powerinfo.ReadObservation()
	if err != nil {
		return localStatusView{}, err
	}
	charging, err := d.smc.IsChargingEnabled()
	if err != nil {
		return localStatusView{}, err
	}
	adapter, err := d.smc.IsAdapterEnabled()
	if err != nil {
		return localStatusView{}, err
	}
	plugged, err := d.smc.IsPluggedIn()
	if err != nil {
		return localStatusView{}, err
	}
	network, err := localipc.OwnSocketCounts()
	if err != nil {
		return localStatusView{}, err
	}
	return localStatusView{o, d.conf.UpperLimit(), d.conf.LowerLimit(), d.conf.ChargeOnceTarget(), d.conf.AdapterDisableUntil(), charging, adapter, plugged, network}, nil
}

// CheckAlDenteReleased is the takeover admission gate, run before SMC access.
// Require no competing controller, without requiring a launchd disable override.
// Runtime monitoring is intentionally absent; clients remain authenticated.
func CheckAlDenteReleased() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := localipc.CheckNoAlDenteProcess(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "/bin/launchctl", "print", "system/com.apphousekitchen.aldente-pro.helper")
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	if output, err := cmd.CombinedOutput(); err == nil || (!strings.Contains(string(output), "Could not find service") && !strings.Contains(string(output), "not found")) {
		return errors.New("AlDente helper is loaded or its state cannot be verified")
	}
	return nil
}

// controlRoutes exposes only the three approved controls and local status.
// The caller serializes these handlers with maintenance and handoff checks.
func (d *Daemon) controlRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		value, err := d.localStatus()
		if err != nil {
			d.fail(w, 503, err)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("PUT /limit", func(w http.ResponseWriter, r *http.Request) {
		var limit int
		if err := decodeJSON(r, &limit); err != nil || limit < 50 || limit > 100 {
			d.fail(w, 400, errors.New("limit must be between 50 and 100"))
			return
		}
		b, _ := json.Marshal(limit)
		r.Body = ioNopCloser(string(b))
		d.setLimit(w, r)
	})
	mux.HandleFunc("POST /charge-once/full", d.postChargeOnceToFull)
	mux.HandleFunc("POST /charge-once/cancel", d.postCancelChargeOnce)
	mux.HandleFunc("POST /discharge/start", func(w http.ResponseWriter, r *http.Request) {
		charge, err := d.smc.GetBatteryCharge()
		if err != nil || charge <= 30 {
			d.fail(w, 409, errors.New("discharge requires battery above 30%"))
			return
		}
		plugged, err := d.smc.IsPluggedIn()
		if err != nil || !plugged {
			d.fail(w, 409, errors.New("connect external power before manual discharge"))
			return
		}
		closed, err := powerinfo.LidClosed()
		if err != nil || closed {
			d.fail(w, 409, errors.New("open the laptop lid before manual discharge"))
			return
		}
		// Bound every session to one hour. Restart/sleep also cancels it.
		r.Body = ioNopCloser(`"1h"`)
		d.setAdapterDisableFor(w, r)
	})
	mux.HandleFunc("POST /discharge/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := d.stopLocalDischarge(); err != nil {
			d.fail(w, 503, err)
			return
		}
		if !d.maintainLoopForced() {
			d.fail(w, 503, errors.New("adapter restored but charging state needs verification"))
			return
		}
		writeJSON(w, 200, "adapter restored")
	})
	return recoverPanics(mux)
}

// ReadOnly strings reader with Close, for reuse of the existing strict handlers.
type stringBody struct{ *strings.Reader }

func (stringBody) Close() error       { return nil }
func ioNopCloser(s string) stringBody { return stringBody{strings.NewReader(s)} }

func (d *Daemon) stopLocalDischarge() error {
	d.transitionMu.Lock()
	defer d.transitionMu.Unlock()
	if err := d.enableAdapter(); err != nil {
		return err
	}
	d.conf.ClearAdapterDisableTimer()
	return d.conf.Save()
}

type localSleepHandler struct {
	d       *Daemon
	gate    *sync.Mutex
	service *localService
}

func (h localSleepHandler) CanSystemSleep() bool { return true }
func (h localSleepHandler) SystemWillSleep() {
	h.gate.Lock()
	defer h.gate.Unlock()
	if h.service != nil {
		h.service.safety.sleepBlocked.Store(true)
		h.service.safety.invalidate()
		h.service.onSleep()
	}
	if !h.d.conf.AdapterDisableUntil().IsZero() {
		if err := h.d.stopLocalDischarge(); err != nil {
			slog.Error("sleep power restoration failed", "error", err)
		}
	}
	if h.service != nil {
		h.service.sleep.manualUntil = time.Time{}
		_ = h.service.releaseLocalSleep()
	}
	// Do not manipulate global sleep settings or suppress lid-close sleep.
	h.d.localSleeping = true
	if h.service == nil || h.service.preferences.Sleep.StopCharging {
		if err := h.d.smc.DisableCharging(); err != nil {
			slog.Error("pre-sleep charging pause failed", "error", err)
		}
	}
}
func (h localSleepHandler) SystemHasPoweredOn() {
	h.gate.Lock()
	defer h.gate.Unlock()
	h.d.localSleeping = false
	if h.service != nil {
		h.service.safety.invalidate()
		h.service.reschedule(time.Now(), "唤醒后重新排期")
		h.service.nextMaintain = time.Time{}
	}
}

// RunLocalControl is the only phase-2 hardware entry. All gates run before SMC
// writes. There is no TCP listener, shell command API, update or upload route.
func RunLocalControl(ctx context.Context) (runErr error) {
	if os.Geteuid() != 0 {
		return errors.New("the protected helper must run as root")
	}
	p, err := localipc.LoadPolicy(localipc.DefaultPolicyPath)
	if err != nil {
		return err
	}
	if err := p.VerifyXPCServer(); err != nil {
		return err
	}
	if err := CheckAlDenteReleased(); err != nil {
		return err
	}
	// Admission only: never apply laptop charge controls on a desktop or a
	// system whose built-in battery cannot be read.
	if _, err := powerinfo.ReadObservation(); err != nil {
		return err
	}
	c, err := config.NewFile(filepath.Join(LocalStateDir, "control.json"))
	if err != nil {
		return err
	}
	// Only the limit persists. Never resume unattended discharge or a full
	// override after a crash, reboot or upgrade. Disable all out-of-scope knobs.
	if c.UpperLimit() < 50 || c.UpperLimit() > 100 {
		c.SetUpperLimit(80)
	}
	if gap := c.UpperLimit() - c.LowerLimit(); gap < 2 || gap > 10 {
		c.SetLowerLimit(c.UpperLimit() - 2)
	}
	c.SetPreventIdleSleep(false)
	c.SetDisableChargingPreSleep(false)
	c.SetPreventSystemSleep(false)
	c.SetPreventSleepOnAdapterDisable(false)
	c.SetAllowNonRootAccess(false)
	c.SetAdapterMode(false)
	c.SetNoDrain(false)
	c.SetControlMagSafeLED(config.ControlMagSafeModeDisabled)
	c.SetCron("")
	c.ClearDisableTimer()
	c.ClearAdapterDisableTimer()
	c.ClearChargeOnceTarget()
	if err := c.Save(); err != nil {
		return err
	}
	s := smc.New()
	if err := s.Open(); err != nil {
		return err
	}
	d := New(c, darwinSystem(s))
	// Read-only hardware probe first. Do not fall back to adapter cycling.
	if s.ChargeControlMode() != compatibility.ChargeControlLegacy || !s.IsAdapterControlCapable() {
		s.Close()
		return errors.New("this firmware does not support the required direct charge/adapter controls")
	}
	if err := d.start(LocalStateDir); err != nil {
		s.Close()
		return err
	}
	service, err := newLocalService(d, LocalStateDir)
	if err != nil {
		_ = d.shutdown()
		return err
	}
	defer func() { runErr = errors.Join(runErr, service.releaseLocalSleep(), d.shutdown()) }()
	// AlDente may have left charge inhibition behind. The old controller is
	// now verified stopped; restore adapter supply before applying our limit.
	if err := d.stopLocalDischarge(); err != nil {
		return err
	}
	if err := service.inspectSafety(); err != nil {
		return err
	}
	if _, err := service.updateSleep(time.Now()); err != nil {
		service.lastError = err.Error()
	}
	if !service.safety.Paused && !service.safety.sleepBlocked.Load() && !service.calibrationBlocked() {
		if err := d.smc.EnableCharging(); err != nil {
			return err
		}
	}
	var gate sync.Mutex
	stopped := false
	routes := recoverPanics(service.routes(d.controlRoutes()))

	stopXPC, err := localxpc.Listen(p, func(command string, payload []byte) (int, []byte) {
		gate.Lock()
		defer gate.Unlock()
		if stopped {
			return 503, []byte("control stopped")
		}
		method, path, ok := localxpc.Endpoint(command)
		if !ok {
			return 400, []byte("unsupported command")
		}
		if method != "GET" && command != "stop-discharge" && command != "allow-sleep" && command != "cancel-calibration" {
			if err := service.inspectSafety(); err != nil {
				return 503, []byte("safety data could not be refreshed")
			}
			if _, err := service.updateSleep(time.Now()); err != nil {
				service.lastError = err.Error()
			}
		}
		// Preserve validated business handlers in-process. This is not a network transport.
		r, err := http.NewRequest(method, "http://in-process"+path, bytes.NewReader(payload))
		if err != nil {
			return 400, []byte("invalid command")
		}
		r.Header.Set("Content-Type", "application/json")
		w := &localActionResult{}
		routes.ServeHTTP(w, r)
		code := w.code
		if code == 0 {
			code = 200
		}
		return code, w.body.Bytes()
	})
	if err != nil {
		return err
	}
	defer stopXPC()
	serveErr := make(chan error, 1)
	go func() {
		if err := d.notifier.Listen(localSleepHandler{d: d, gate: &gate, service: service}); err != nil {
			serveErr <- err
		}
	}()
	defer func() { d.notifier.Stop(); gate.Lock(); stopped = true; gate.Unlock() }()
	tickContext, cancelTicks := context.WithCancel(ctx)
	defer cancelTicks()
	ticks := localCalendarTicks(tickContext)
	for {
		gate.Lock()
		var err error
		if !c.AdapterDisableUntil().IsZero() {
			charge, readErr := s.GetBatteryCharge()
			if readErr != nil || charge <= 30 {
				err = d.stopLocalDischarge()
			}
		}
		if err == nil {
			err = service.tick(time.Now())
		}
		gate.Unlock()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-serveErr:
			return err
		case <-ticks:
		}
	}
}
