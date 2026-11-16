// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charlie0129/batt/pkg/localipc"
	"github.com/charlie0129/batt/pkg/localxpc"
	"github.com/charlie0129/batt/pkg/observer"
)

func TestLocalReleasePinsExactHashes(t *testing.T) {
	c := localipc.Config{SchemaVersion: 2, AuthorizedUID: 501, ServerIdentifier: "io.nero.battery.helper", ClientIdentifiers: []string{"io.nero.battery.app"}, ServerCDHash: strings.Repeat("a", 40), ClientCDHash: strings.Repeat("b", 40)}
	if _, err := localipc.NewPolicy(c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*localipc.Config){
		func(c *localipc.Config) { c.ClientCDHash = "" },
		func(c *localipc.Config) { c.ClientCDHash = c.ServerCDHash },
		func(c *localipc.Config) { c.ServerCDHash = `a" or true` },
		func(c *localipc.Config) { c.AuthorizedUID = 0 },
		func(c *localipc.Config) { c.ClientIdentifiers = []string{"io.nero.battery.app", "unapproved"} },
	} {
		bad := c
		mutate(&bad)
		if _, err := localipc.NewPolicy(bad); err == nil {
			t.Fatal("unsafe local policy accepted")
		}
	}
}

func TestLocalNetworkInspectionSeesOwnInternetSocket(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	counts, err := localipc.OwnSocketCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Internet < 1 {
		t.Fatal("native inspection missed this process's known Internet-family socket")
	}
}

func TestLocalThermalPolicyHasHysteresisAndHardwareGuard(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	safety := &localSafety{}
	p := localThermalSettings{true, 40, 37}
	hot := 41.0
	safety.decide(p, &hot, 0)
	guard := localSafeSMC{SMC: f.smc, safety: safety, readCharge: f.smc.GetBatteryCharge}
	if guard.EnableCharging() != errLocalThermalPause || guard.DisableAdapter() != errLocalThermalPause {
		t.Fatal("thermal protection did not guard both write paths")
	}
	warm := 38.0
	safety.decide(p, &warm, 0)
	if !safety.Paused {
		t.Fatal("charging resumed before crossing recovery temperature")
	}
	cool := 36.0
	safety.decide(p, &cool, 0)
	if safety.Paused {
		t.Fatal("normal temperature did not resume policy")
	}
	safety.decide(p, nil, 0)
	if !safety.Paused {
		t.Fatal("missing battery sensor did not fail safely")
	}
	safety.decide(p, &cool, 2)
	if !safety.Paused {
		t.Fatal("serious system thermal pressure was ignored")
	}
}

func TestLocalScheduleRunsOnceAndSkipsMissedJobs(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	s, err := newLocalService(f.d, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	s.preferences.Thermal.Enabled = false
	now := time.Now()
	rule := localSchedule{ID: "limit", Enabled: true, Action: "limit", Hour: now.Hour(), Minute: now.Minute(), Days: "*", Limit: 95}
	s.preferences.Schedules = []localSchedule{rule}
	s.nextRules[rule.ID] = now.Add(-time.Second)
	s.runRules(now)
	saves := f.mem.saves
	if f.conf.UpperLimit() != 95 || saves == 0 || s.preferences.Schedules[0].Result != "已完成" {
		t.Fatal("scheduled limit was not applied")
	}
	s.runRules(now)
	if f.mem.saves != saves {
		t.Fatal("a completed job ran twice")
	}
	s.preferences.Schedules[0].Limit = 80
	s.nextRules[rule.ID] = now.Add(-3 * time.Minute)
	s.runRules(now)
	if f.conf.UpperLimit() != 95 || !strings.HasPrefix(s.preferences.Schedules[0].Result, "已跳过") {
		t.Fatal("an old job was replayed after its safe window")
	}
}

func TestLocalControlScopeAndChargeOverride(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	h := f.d.controlRoutes()
	call := func(method, path, body string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/adapter", "/schedule", "/no-drain", "/prevent-system-sleep", "/magsafe-led", "/adapter-mode"} {
		call("PUT", path, "true", 404)
	}
	call("POST", "/calibration/start", "", 404)
	call("PUT", "/limit", "49", 400)
	call("PUT", "/limit", "65", 201)
	if f.conf.UpperLimit() != 65 {
		t.Fatal("limit not applied")
	}
	call("POST", "/charge-once/full", "", 201)
	if f.conf.ChargeOnceTarget() != 100 {
		t.Fatal("full override not set")
	}
	call("POST", "/charge-once/cancel", "", 200)
	if f.conf.ChargeOnceTarget() != 0 || f.conf.UpperLimit() != 65 {
		t.Fatal("cancel lost saved limit")
	}
	f.smc.setCharge(t, 30)
	call("POST", "/discharge/start", "", 409)
	if !f.smc.adapterEnabled(t) {
		t.Fatal("low battery disconnected adapter")
	}
}

func TestLocalSleepRestoresDischargeWithoutSuppressingSleep(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	f.conf.SetPreventSleepOnAdapterDisable(false)
	f.conf.SetAdapterDisableTimer(time.Now().Add(time.Hour))
	f.smc.setAdapter(t, false)
	var gate sync.Mutex
	h := localSleepHandler{d: f.d, gate: &gate}
	if !h.CanSystemSleep() {
		t.Fatal("local controller must allow sleep")
	}
	h.SystemWillSleep()
	if !f.smc.adapterEnabled(t) || !f.conf.AdapterDisableUntil().IsZero() || f.smc.chargingEnabled(t) {
		t.Fatal("sleep did not restore adapter and pause charging")
	}
	h.SystemHasPoweredOn()
	if f.d.localSleeping {
		t.Fatal("wake did not resume maintenance")
	}
}

func TestPhaseOneRejectsControlRequests(t *testing.T) {
	handler := observer.Handler()
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		for _, path := range []string{"/snapshot", "/health", "/limit", "/adapter", "/calibration"} {
			r := httptest.NewRecorder()
			handler.ServeHTTP(r, httptest.NewRequest(method, path, nil))
			if r.Code != http.StatusMethodNotAllowed && r.Code != http.StatusNotFound {
				t.Fatalf("%s %s returned %d; no control endpoint may exist", method, path, r.Code)
			}
		}
	}
	if err := Run("unused-config", "unused-socket", true); err == nil {
		t.Fatal("legacy broad-access flag must fail before config, SMC or endpoint setup")
	}
}

func TestPhaseOneSigningPolicyRejectsUnsafeInputs(t *testing.T) {
	c := localipc.Config{
		SchemaVersion: 1, AuthorizedUID: uint32(os.Geteuid() + 1), TeamID: "ZZZZZZZZZZ",
		ServerIdentifier: "io.nero.battery.observer", ClientIdentifiers: []string{"io.nero.battery.app"},
	}
	if _, err := localipc.NewPolicy(c); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.TeamID = ""
	if _, err := localipc.NewPolicy(bad); err == nil {
		t.Fatal("missing signing identity was accepted")
	}
	bad = c
	bad.ClientIdentifiers = []string{`a" or true or identifier "b`}
	if _, err := localipc.NewPolicy(bad); err == nil {
		t.Fatal("requirement injection was accepted")
	}
	bad = c
	bad.AuthorizedUID = 0
	if _, err := localipc.NewPolicy(bad); err == nil {
		t.Fatal("root must not be the ordinary client identity")
	}
	bad = c
	bad.ClientIdentifiers = []string{"io.nero.battery.app", "io.nero.battery.app"}
	if _, err := localipc.NewPolicy(bad); err == nil {
		t.Fatal("duplicate client identities were accepted")
	}
	if _, err := localipc.Listen("unused-socket", nil); err == nil {
		t.Fatal("missing policy must fail before an endpoint is created")
	}
	missing := fmt.Sprintf("/Library/.nero-battery-unprovisioned-policy-%d.json", os.Getpid())
	if _, err := localipc.LoadPolicy(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("root directory/empty-ACL checks should reach the absent policy, got %v", err)
	}
}

func TestPhaseOnePeerAccountUsesKernelAuditToken(t *testing.T) {
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: t.TempDir() + "/peer.sock", Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	client, err := net.DialUnix("unix", nil, l.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := l.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	c := localipc.Config{SchemaVersion: 1, AuthorizedUID: uint32(os.Geteuid() + 1), TeamID: "ZZZZZZZZZZ",
		ServerIdentifier: "io.nero.battery.observer", ClientIdentifiers: []string{"io.nero.battery.no-match"}}
	p, err := localipc.NewPolicy(c)
	if err != nil {
		t.Fatal(err)
	}
	var rejected *localipc.VerificationError
	if err := p.VerifyPeer(server); !errors.As(err, &rejected) || rejected.Stage != 2 {
		t.Fatalf("wanted kernel-audit-token UID rejection, got %v", err)
	}
	c.AuthorizedUID = uint32(os.Geteuid())
	if c.AuthorizedUID == 0 {
		t.Skip("release clients intentionally cannot be root")
	}
	p, err = localipc.NewPolicy(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.VerifyPeer(server); !errors.As(err, &rejected) || rejected.Stage < 3 {
		t.Fatalf("a nonmatching release identity must fail after token/UID validation, got %v", err)
	}
	t.Logf("unsigned or nonmatching test client rejected at signing stage %d", rejected.Stage)
}

func TestRemoveStaleSocket(t *testing.T) {
	dir := t.TempDir()

	// Nonexistent path is a no-op.
	if err := removeStaleSocket(dir + "/none.sock"); err != nil {
		t.Fatalf("missing socket should be ok, got %v", err)
	}

	// A non-socket file is refused, not deleted.
	plain := dir + "/plain"
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(plain); err == nil {
		t.Fatal("must refuse to remove a non-socket file")
	}
	if _, err := os.Stat(plain); err != nil {
		t.Fatal("non-socket file must be left intact")
	}

	// A socket with a live listener is refused (don't clobber a running daemon).
	live := dir + "/live.sock"
	ll, err := net.Listen("unix", live)
	if err != nil {
		t.Fatal(err)
	}
	defer ll.Close()
	if err := removeStaleSocket(live); err == nil {
		t.Fatal("must refuse to remove a socket with a live listener")
	}

	// A stale socket file (no listener) is removed. SetUnlinkOnClose(false)
	// leaves the file behind on Close, emulating a SIGKILLed daemon.
	stale := dir + "/stale.sock"
	sl, err := net.ListenUnix("unix", &net.UnixAddr{Name: stale, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	sl.SetUnlinkOnClose(false)
	_ = sl.Close()
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("precondition: stale socket file should exist, got %v", err)
	}
	if err := removeStaleSocket(stale); err != nil {
		t.Fatalf("stale socket should be removed, got %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale socket file should be gone")
	}
}

func TestLocalSleepBoundariesAndOneTimeScheduling(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	s, err := newLocalService(f.d, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.sleep.manualUntil = now.Add(time.Hour)
	if err := s.reconcileSleep(now, false, true, 70); err != nil || !s.sleep.Active {
		t.Fatal("awake hold missing", err)
	}
	if err := s.reconcileSleep(now, true, true, 70); err != nil || s.sleep.Active || f.asserts.onAC {
		t.Fatal("lid close kept hold", err)
	}
	for _, boundary := range []struct {
		plug   bool
		charge int
	}{{false, 70}, {true, 20}} {
		s.sleep.manualUntil = now.Add(time.Hour)
		if err := s.reconcileSleep(now, false, boundary.plug, boundary.charge); err != nil || s.sleep.Active {
			t.Fatal("safety did not release", err)
		}
	}
	s.sleep.manualUntil = now.Add(-time.Second)
	_ = s.reconcileSleep(now, false, true, 70)
	if s.sleep.Active {
		t.Fatal("expired hold retained")
	}
	s.preferences.Sleep.UntilLimit = true
	s.preferences.Sleep.MaxMinutes = 1
	_ = s.reconcileSleep(now, false, true, 70)
	_ = s.reconcileSleep(now.Add(2*time.Minute), false, true, 70)
	if s.sleep.Active {
		t.Fatal("automatic hold exceeded bound")
	}
	once := now.Add(-time.Second)
	s.preferences.Schedules = []localSchedule{{ID: "once", Enabled: true, Action: "limit", Limit: 90, Days: "*", OnceAt: &once}}
	s.refreshRules(now)
	s.runRules(now)
	if f.conf.UpperLimit() != 90 || s.preferences.Schedules[0].Enabled {
		t.Fatal("one-time schedule did not complete and disable")
	}
	saves := f.mem.saves
	s.refreshRules(now.Add(time.Second))
	s.runRules(now.Add(time.Second))
	if f.mem.saves != saves {
		t.Fatal("one-time task replayed")
	}
	weekly := localSchedule{ID: "weekly", Action: "limit", Limit: 80, Days: "1,3,5", Hour: 10, Minute: 30}
	spec, err := weekly.spec()
	if err != nil || !spec.Next(now).After(now) {
		t.Fatal("native calendar schedule invalid", err)
	}
	if _, err = (localSchedule{Action: "limit", Limit: 80, Days: "1,1"}).spec(); err == nil {
		t.Fatal("duplicate weekday accepted")
	}
}

func TestXPCCommandAndSigningBoundaries(t *testing.T) {
	for _, path := range []string{"/arbitrary", "/status?path=/etc/passwd", "/history?range=all", "/adapter"} {
		if _, err := localxpc.Command("GET", path); err == nil {
			t.Fatal("unsupported route accepted", path)
		}
	}
	for _, pair := range [][2]string{{"GET", "/status"}, {"PUT", "/band"}, {"GET", "/history?range=24h"}, {"PUT", "/sleep-policy"}} {
		command, err := localxpc.Command(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		m, p, ok := localxpc.Endpoint(command)
		if !ok || m != pair[0] || p != pair[1] {
			t.Fatal("command did not round-trip")
		}
	}
	var missing *localipc.Policy
	if _, _, _, err := missing.XPCIdentity(); err == nil {
		t.Fatal("missing signing policy accepted")
	}
	p, err := localipc.NewPolicy(localipc.Config{SchemaVersion: 2, AuthorizedUID: 501, ServerIdentifier: "io.nero.battery.helper", ClientIdentifiers: []string{"io.nero.battery.app"}, ServerCDHash: strings.Repeat("a", 40), ClientCDHash: strings.Repeat("b", 40)})
	if err != nil {
		t.Fatal(err)
	}
	uid, server, client, err := p.XPCIdentity()
	if err != nil || uid != 501 || !strings.Contains(server, strings.Repeat("a", 40)) || !strings.Contains(client, strings.Repeat("b", 40)) {
		t.Fatal("XPC lost exact identity policy")
	}
}
func TestInstalledXPCRejectsUnapprovedReadOnlyClient(t *testing.T) {
	if os.Getenv("NERO_VERIFY_INSTALLED_XPC") != "1" {
		t.Skip("requires the authorized installed formal release")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(filepath.Join(home, "Applications/NERO Battery.app/Contents/MacOS/nero-battery"), "--status").Output()
	if err != nil || !strings.Contains(string(out), "NSXPCConnection") {
		t.Fatal("approved installed XPC client is not healthy", err)
	}
	_, err = localxpc.Request("status", nil)
	if err == nil || strings.Contains(err.Error(), "timed out") {
		t.Fatal("unapproved test binary was not promptly rejected", err)
	}
	t.Log("Healthy approved client succeeded; unapproved status-only client rejected:", err)
}

func TestLocalReportedChargeAndHonestSleepRelease(t *testing.T) {
	f := newFixture(t, firmwareTahoe, withCharge(70))
	missing := false
	f.d.localCharge = func() (int, error) {
		if missing {
			return 0, errors.New("missing reported percentage")
		}
		return 95, nil
	}
	s, err := newLocalService(f.d, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	charge, err := s.d.smc.GetBatteryCharge()
	if err != nil || charge != 95 {
		t.Fatal("control did not use reported charge", charge, err)
	}
	raw, _ := s.rawSMC.GetBatteryCharge()
	if raw != 70 {
		t.Fatal("raw source unexpectedly replaced")
	}
	s.preferences.Thermal.Enabled = false
	s.safety.decide(s.preferences.Thermal, nil, 0)
	if err := s.applyBand(95, 93); err != nil {
		t.Fatal(err)
	}
	if f.smc.chargingEnabled(t) {
		t.Fatal("control followed raw rather than displayed percentage")
	}
	s.sleep.Active = true
	s.sleep.Until = time.Now().Add(time.Hour)
	f.asserts.err = errors.New("release failed")
	if err := s.releaseLocalSleep(); err == nil || !s.sleep.Active || !s.sleep.ReleasePending || s.sleep.Until.IsZero() {
		t.Fatal("release failure reported success")
	}
	f.asserts.err = nil
	if err := s.releaseLocalSleep(); err != nil || s.sleep.Active || s.sleep.ReleasePending || !s.sleep.Until.IsZero() {
		t.Fatal("successful retry did not clear pending state")
	}
	missing = true
	if _, err := s.d.smc.GetBatteryCharge(); err == nil {
		t.Fatal("missing percent silently fell back to raw SMC")
	}
}
func TestLocalStaleAndSleepProtectionGuardEveryWrite(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	p := localThermalSettings{true, 40, 37}
	cool := 36.0
	guard := localSafeSMC{SMC: f.smc, safety: &localSafety{}, readCharge: f.smc.GetBatteryCharge}
	guard.safety.decide(p, &cool, 0)
	guard.safety.sampledUnix.Store(time.Now().Add(-20 * time.Second).UnixNano())
	if guard.EnableCharging() != errLocalStaleSafety || guard.DisableAdapter() != errLocalStaleSafety {
		t.Fatal("stale data allowed active control")
	}
	if err := guard.EnableAdapter(); err != nil {
		t.Fatal("safe external-power restore was blocked", err)
	}
	guard.safety.decide(p, &cool, 0)
	guard.safety.sleepBlocked.Store(true)
	if guard.EnableCharging() != errLocalSleepPause || guard.DisableAdapter() != errLocalSleepPause {
		t.Fatal("sleep pause failed to protect direct writes")
	}
	if err := guard.DisableCharging(); err != nil {
		t.Fatal("safe charging pause was blocked", err)
	}
}
func TestLocalCalendarRecoveryDoesNotReplay(t *testing.T) {
	f := newFixture(t, firmwareTahoe)
	s, err := newLocalService(f.d, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	past := now.Add(-time.Minute)
	s.preferences.Schedules = []localSchedule{{ID: "once", Enabled: true, Action: "sleep", Days: "*", OnceAt: &past}}
	s.nextRules["once"] = past
	s.reschedule(now, "唤醒后重新排期")
	if s.preferences.Schedules[0].Enabled || !strings.HasPrefix(s.preferences.Schedules[0].Result, "已跳过") {
		t.Fatal("missed one-time task remained armed")
	}
	spec := localCalendarSchedule{hour: 1, minute: 30, weekdays: 127, zone: "America/New_York"}
	first, _ := time.Parse(time.RFC3339, "2026-11-01T01:30:00-04:00")
	after := first.Add(time.Minute)
	next := nextRuleOccurrence(spec, localSchedule{LastRun: first}, after)
	loc, _ := time.LoadLocation("America/New_York")
	if next.In(loc).Day() != 2 {
		t.Fatal("DST repeated-hour replay", next.In(loc))
	}
	future := now.Add(time.Hour)
	s.preferences.Schedules = []localSchedule{{ID: "daily", Enabled: true, Action: "sleep", Days: "*", Hour: now.Hour(), Minute: now.Minute(), LastRun: future}}
	s.refreshRules(now)
	if !s.nextRules["daily"].After(future) {
		t.Fatal("clock rollback replayed an executed task")
	}
}
