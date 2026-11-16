// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/charlie0129/batt/pkg/calibration"
	"github.com/charlie0129/batt/pkg/powerinfo"
)

type localService struct {
	sleep        localSleepState
	d            *Daemon
	rawSMC       SMC
	dir          string
	preferences  localPreferences
	safety       *localSafety
	history      *localHistory
	nextRules    map[string]time.Time
	nextMaintain time.Time
	clockLast    time.Time
	clockZone    string
	lastError    string
	energy       localEnergyReport
	mode         localPowerMode
}

func newLocalService(d *Daemon, dir string) (*localService, error) {
	p, err := loadLocalPreferences(dir)
	if err != nil {
		return nil, err
	}
	if d.localCharge == nil || d.localLid == nil {
		return nil, errors.New("local telemetry readers required")
	}
	s := &localService{d: d, rawSMC: d.smc, dir: dir, preferences: p, safety: &localSafety{}, history: &localHistory{dir: filepath.Join(dir, "history")}, nextRules: map[string]time.Time{}}
	d.smc = localSafeSMC{SMC: d.smc, safety: s.safety, readCharge: d.localCharge}
	for i := range s.preferences.Schedules {
		if s.preferences.Schedules[i].Result == "正在执行" {
			s.preferences.Schedules[i].Result = "上次任务未留下完成回执，请核对当前状态"
		}
	}
	s.refreshRules(time.Now())
	s.mode = readLocalPowerMode()
	return s, nil
}

func (s *localService) savePreferences(p localPreferences) error {
	if err := p.validate(); err != nil {
		return err
	}
	if err := writePrivateJSON(filepath.Join(s.dir, "preferences.json"), p); err != nil {
		return err
	}
	s.preferences = p
	s.refreshRules(time.Now())
	return nil
}

func (s *localService) tick(now time.Time) error {
	if err := s.inspectSafety(); err != nil {
		return err
	}
	blocked := s.calibrationBlocked()
	if blocked {
		if err := s.d.smc.EnableAdapter(); err != nil {
			return err
		}
		if err := s.d.smc.DisableCharging(); err != nil {
			return err
		}
	}
	lidPaused, err := s.updateSleep(now)
	if err != nil {
		s.lastError = err.Error()
	}
	s.checkClock(now)
	s.runRules(now)
	if !s.safety.Paused && !blocked && !lidPaused && !s.d.localSleeping && !now.Before(s.nextMaintain) {
		s.d.loopOnce(now)
		s.nextMaintain = now.Add(10 * time.Second)
	}
	if lidPaused {
		if err := s.d.smc.DisableCharging(); err != nil {
			return err
		}
	}
	if !now.Before(s.history.next) {
		if o, err := powerinfo.ReadObservation(); err == nil {
			s.history.record(now, o, s.safety.Temperature, s.d.conf.UpperLimit())
		}
	}
	return nil
}

func (s *localService) status(w http.ResponseWriter, r *http.Request) {
	if s.safety.enabled.Load() && s.safety.controlError() == errLocalStaleSafety {
		s.safety.Valid = false
		s.safety.Temperature = nil
		s.safety.Reason = "保护数据已过期，等待刷新"
	}
	base, err := s.d.localStatus()
	if err != nil {
		s.d.fail(w, 503, err)
		return
	}
	effective := base.Limit
	s.d.calibrationMu.Lock()
	calibrationState := *s.d.calibrationState
	if st := s.d.calibrationState; st.Phase != calibration.PhaseIdle {
		base.Limit = st.SnapshotUpperLimit
		base.Lower = st.SnapshotLowerLimit
	}
	s.d.calibrationMu.Unlock()
	preferences := s.preferences
	preferences.Schedules = append([]localSchedule{}, s.preferences.Schedules...)
	for i := range preferences.Schedules {
		preferences.Schedules[i].NextRun = s.nextRules[preferences.Schedules[i].ID]
	}
	var rawSMC *int
	if v, err := s.rawSMC.GetBatteryCharge(); err == nil {
		rawSMC = &v
	}
	writeJSON(w, 200, struct {
		localStatusView
		Sleep            localSleepState     `json:"sleep"`
		EffectiveLimit   int                 `json:"effective_limit"`
		Safety           *localSafety        `json:"safety"`
		Calibration      *calibration.Status `json:"calibration"`
		CalibrationState calibration.State   `json:"calibration_state"`
		Preferences      localPreferences    `json:"preferences"`
		Energy           localEnergyReport   `json:"energy"`
		Mode             localPowerMode      `json:"power_mode"`
		HistoryAt        time.Time           `json:"history_at"`
		HistoryError     string              `json:"history_error"`
		LastError        string              `json:"last_error"`
		RawSMC           *int                `json:"raw_smc_charge_percent"`
		ChargeBasis      string              `json:"charge_basis"`
	}{base, s.sleep, effective, s.safety, s.d.getCalibrationStatus(), calibrationState, preferences, s.energy, s.mode, s.history.LastAt, s.history.LastError, s.lastError, rawSMC, "macOS reported percent"})
}

func (s *localService) routes(base http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", base)
	s.sleepRoutes(mux)
	mux.HandleFunc("GET /status", s.status)
	mux.HandleFunc("PUT /thermal", func(w http.ResponseWriter, r *http.Request) {
		var value localThermalSettings
		if err := decodeJSON(r, &value); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		p := s.preferences
		p.Thermal = value
		if err := s.savePreferences(p); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		if err := s.inspectSafety(); err != nil {
			s.d.fail(w, 503, err)
			return
		}
		writeJSON(w, 200, "thermal policy saved")
	})
	mux.HandleFunc("PUT /band", s.setBand)
	mux.HandleFunc("PUT /limit", func(w http.ResponseWriter, r *http.Request) {
		var upper int
		if err := decodeJSON(r, &upper); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		delta := s.d.conf.UpperLimit() - s.d.conf.LowerLimit()
		if err := s.applyBand(upper, upper-delta); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		writeJSON(w, 200, "limit saved")
	})
	mux.HandleFunc("GET /history", func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("range")
		if period == "" {
			period = "24h"
		}
		points, err := s.history.read(time.Now(), period)
		if err != nil {
			s.d.fail(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"range": period, "retention_days": historyRetentionDays, "sample_interval_seconds": 30, "points": points, "error": s.history.LastError})
	})
	mux.HandleFunc("PUT /schedules", func(w http.ResponseWriter, r *http.Request) {
		var rules []localSchedule
		if err := decodeJSON(r, &rules); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		// The client chooses schedules, not execution receipts.
		previous := map[string]localSchedule{}
		for _, item := range s.preferences.Schedules {
			previous[item.ID] = item
		}
		for i := range rules {
			rules[i].NextRun = time.Time{}
			if old, ok := previous[rules[i].ID]; ok {
				rules[i].LastRun = old.LastRun
				if rules[i].Action != old.Action || rules[i].Hour != old.Hour || rules[i].Minute != old.Minute || rules[i].Days != old.Days || rules[i].Limit != old.Limit || !sameLocalDate(rules[i].OnceAt, old.OnceAt) {
					rules[i].LastRun = time.Time{}
					rules[i].Result = "修改后待执行"
				}
				rules[i].Result = old.Result
			} else {
				rules[i].LastRun = time.Time{}
				rules[i].Result = ""
			}
		}
		p := s.preferences
		p.Schedules = rules
		if err := s.savePreferences(p); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		writeJSON(w, 200, "schedules saved")
	})
	mux.HandleFunc("POST /calibration/start", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Threshold int `json:"threshold"`
			Hold      int `json:"hold_minutes"`
		}
		if err := decodeJSON(r, &p); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		if err := s.startLocalCalibration(p.Threshold, p.Hold); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		writeJSON(w, 200, "calibration started")
	})
	mux.HandleFunc("POST /calibration/pause", func(w http.ResponseWriter, r *http.Request) {
		if err := s.d.pauseCalibration(); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		if err := s.d.smc.EnableAdapter(); err != nil {
			s.d.fail(w, 503, err)
			return
		}
		if err := s.d.smc.DisableCharging(); err != nil {
			s.d.fail(w, 503, err)
			return
		}
		writeJSON(w, 200, "calibration paused; external power restored")
	})
	mux.HandleFunc("POST /calibration/resume", func(w http.ResponseWriter, r *http.Request) {
		if err := s.requireReady(); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		if err := s.d.resumeCalibration(); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		s.nextMaintain = time.Time{}
		writeJSON(w, 200, "calibration resumed")
	})
	mux.HandleFunc("POST /calibration/cancel", func(w http.ResponseWriter, r *http.Request) {
		if err := s.d.cancelCalibration(); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		if !s.safety.Paused {
			s.d.maintainLoopForced()
		}
		writeJSON(w, 200, "calibration cancelled; original limits restored")
	})
	mux.HandleFunc("GET /power-mode", func(w http.ResponseWriter, r *http.Request) { s.mode = readLocalPowerMode(); writeJSON(w, 200, s.mode) })
	mux.HandleFunc("PUT /power-mode", func(w http.ResponseWriter, r *http.Request) {
		var mode string
		if err := decodeJSON(r, &mode); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		if err := setLocalPowerMode(mode); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		s.mode = readLocalPowerMode()
		writeJSON(w, 200, s.mode)
	})
	mux.HandleFunc("POST /energy/sample", func(w http.ResponseWriter, r *http.Request) {
		s.energy = sampleLocalEnergy()
		writeJSON(w, 200, s.energy)
	})
	return mux
}

func (s *localService) requireReady() error {
	if s.safety.Paused {
		return errLocalThermalPause
	}
	closed, err := s.d.localLid()
	if err != nil || closed {
		return errors.New("open the laptop lid before calibration")
	}
	plugged, err := s.d.smc.IsPluggedIn()
	if err != nil || !plugged {
		return errors.New("connect external power before calibration")
	}
	return nil
}

func (s *localService) startLocalCalibration(threshold, hold int) error {
	if threshold < 15 || threshold > 30 || hold < 10 || hold > 180 {
		return errors.New("calibration requires 15–30% discharge target and 10–180 minutes hold")
	}
	if err := s.requireReady(); err != nil {
		return err
	}
	if err := s.d.startCalibration(threshold, hold); err != nil {
		return err
	}
	s.nextMaintain = time.Time{}
	return nil
}

func (s *localService) setBand(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Upper int `json:"upper"`
		Lower int `json:"lower"`
	}
	if err := decodeJSON(r, &p); err != nil {
		s.d.fail(w, 400, err)
		return
	}
	if err := s.applyBand(p.Upper, p.Lower); err != nil {
		s.d.fail(w, 400, err)
		return
	}
	writeJSON(w, 200, "charge band saved")
}

func (s *localService) applyBand(upper, lower int) error {
	if upper < 50 || upper > 100 || upper-lower < 2 || upper-lower > 10 {
		return errors.New("upper limit must be 50–100%; restart gap must be 2–10 points")
	}
	d := s.d
	d.transitionMu.Lock()
	if d.calibrationOwnsChargeLimit() {
		d.transitionMu.Unlock()
		return ErrCalibrationControlsChargeLimit
	}
	oldUpper, oldLower, target := d.conf.UpperLimit(), d.conf.LowerLimit(), d.conf.ChargeOnceTarget()
	d.conf.SetUpperLimit(upper)
	d.conf.SetLowerLimit(lower)
	d.conf.ClearChargeOnceTarget()
	if err := d.conf.Save(); err != nil {
		d.conf.SetUpperLimit(oldUpper)
		d.conf.SetLowerLimit(oldLower)
		d.conf.SetChargeOnceTarget(target)
		d.transitionMu.Unlock()
		return err
	}
	d.transitionMu.Unlock()
	if s.safety.Paused || s.safety.sleepBlocked.Load() {
		return errors.New("limit saved, but protection paused application")
	}
	if !d.maintainLoopForced() {
		return errors.New("limit saved, but hardware application failed")
	}
	charge, err := d.smc.GetBatteryCharge()
	if err != nil {
		return errors.New("limit saved, but hardware state not confirmed")
	}
	charging, err := d.smc.IsChargingEnabled()
	if err != nil {
		return errors.New("limit saved, but hardware state not confirmed")
	}
	if charge >= upper && charging || charge <= lower && !charging {
		return errors.New("limit saved, but hardware state not confirmed")
	}
	return nil
}
