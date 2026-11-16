// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"errors"
	"net/http"
	"time"
)

type localSleepPolicy struct {
	StopCharging bool `json:"stop_charging"`
	UntilLimit   bool `json:"until_limit"`
	PauseOnLid   bool `json:"pause_on_lid"`
	MaxMinutes   int  `json:"max_minutes"`
	Floor        int  `json:"floor"`
}
type localSleepState struct {
	Active         bool      `json:"active"`
	Until          time.Time `json:"until"`
	Reason         string    `json:"reason"`
	LidClosed      bool      `json:"lid_closed"`
	ReleasePending bool      `json:"release_pending"`
	manualUntil    time.Time
	autoStarted    time.Time
	autoExpired    bool
	wasPlugged     bool
}
type localSleepAssertions interface {
	PreventLocalSleep() error
	AllowLocalSleep() error
}

func (s *localService) releaseLocalSleep() error {
	if a, ok := s.d.assertions.(localSleepAssertions); ok {
		if err := a.AllowLocalSleep(); err != nil {
			s.sleep.ReleasePending = true
			s.sleep.Reason = "恢复正常睡眠尚未确认，后台将重试"
			return err
		}
	}
	s.sleep.Active = false
	s.sleep.Reason = "正常睡眠"
	s.sleep.ReleasePending = false
	s.sleep.Until = time.Time{}
	return nil
}
func (s *localService) startLocalAwake(minutes int, now time.Time) error {
	if minutes < 1 || minutes > 480 {
		return errors.New("保持唤醒时长须为 1–480 分钟")
	}
	plugged, err := s.d.smc.IsPluggedIn()
	if err != nil || !plugged {
		return errors.New("保持唤醒仅在连接电源时可用")
	}
	lid, err := s.d.localLid()
	if err != nil || lid {
		return errors.New("开盖后才可保持唤醒")
	}
	charge, err := s.d.smc.GetBatteryCharge()
	if err != nil {
		return err
	}
	if charge <= s.preferences.Sleep.Floor || s.safety.Paused {
		return errors.New("电量或热保护条件不允许保持唤醒")
	}
	old := s.sleep.manualUntil
	s.sleep.manualUntil = now.Add(time.Duration(minutes) * time.Minute)
	if err := s.reconcileSleep(now, lid, plugged, charge); err != nil {
		s.sleep.manualUntil = old
		return err
	}
	if !s.sleep.Active {
		return errors.New("保持唤醒未生效")
	}
	return nil
}
func (s *localService) reconcileSleep(now time.Time, lid bool, plugged bool, charge int) error {
	p := s.preferences.Sleep
	s.sleep.LidClosed = lid
	if !plugged || lid || charge <= p.Floor || s.safety.Paused || s.d.localSleeping {
		s.sleep.manualUntil = time.Time{}
		s.sleep.autoStarted = time.Time{}
		s.sleep.autoExpired = false
		s.sleep.wasPlugged = plugged
		s.sleep.Until = time.Time{}
		s.sleep.Reason = "正常睡眠 · 合盖、断电、低电量或热保护会结束保持唤醒"
		return s.releaseLocalSleep()
	}
	if !s.sleep.wasPlugged {
		s.sleep.autoExpired = false
	}
	s.sleep.wasPlugged = plugged
	target := s.d.conf.UpperLimit()
	if t := s.d.conf.ChargeOnceTarget(); t > 0 {
		target = t
	}
	auto := p.UntilLimit && charge < target && !s.sleep.autoExpired
	if auto && s.sleep.autoStarted.IsZero() {
		s.sleep.autoStarted = now
	}
	autoEnd := s.sleep.autoStarted.Add(time.Duration(p.MaxMinutes) * time.Minute)
	if auto && !now.Before(autoEnd) {
		s.sleep.autoExpired = true
		auto = false
	}
	if charge >= target || !p.UntilLimit {
		s.sleep.autoStarted = time.Time{}
		s.sleep.autoExpired = false
		auto = false
	}
	until := s.sleep.manualUntil
	if !now.Before(until) {
		until = time.Time{}
	}
	if auto && autoEnd.After(until) {
		until = autoEnd
	}
	s.sleep.Until = until
	if until.IsZero() {
		s.sleep.Reason = "正常睡眠"
		return s.releaseLocalSleep()
	}
	a, ok := s.d.assertions.(localSleepAssertions)
	if !ok {
		return errors.New("当前后台不支持临时睡眠保持")
	}
	if err := a.PreventLocalSleep(); err != nil {
		return err
	}
	s.sleep.Active = true
	s.sleep.Reason = "保持唤醒中 · 显示器仍可休眠，合盖仍由 macOS 决定"
	return nil
}
func (s *localService) updateSleep(now time.Time) (paused bool, err error) {
	defer func() { s.safety.sleepBlocked.Store(paused || s.d.localSleeping) }()
	lid, err := s.d.localLid()
	if err != nil {
		_ = s.releaseLocalSleep()
		return true, err
	}
	plugged, err := s.d.smc.IsPluggedIn()
	if err != nil {
		_ = s.releaseLocalSleep()
		return true, err
	}
	charge, err := s.d.smc.GetBatteryCharge()
	if err != nil {
		_ = s.releaseLocalSleep()
		return true, err
	}
	if err = s.reconcileSleep(now, lid, plugged, charge); err != nil {
		return true, err
	}
	return lid && plugged && s.preferences.Sleep.PauseOnLid, nil
}
func (s *localService) sleepRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /sleep-policy", func(w http.ResponseWriter, r *http.Request) {
		var p localSleepPolicy
		if err := decodeJSON(r, &p); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		next := s.preferences
		next.Sleep = p
		if err := s.savePreferences(next); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		s.sleep.autoStarted = time.Time{}
		s.sleep.autoExpired = false
		writeJSON(w, 200, "睡眠联动已保存")
	})
	mux.HandleFunc("POST /awake/start", func(w http.ResponseWriter, r *http.Request) {
		var minutes int
		if err := decodeJSON(r, &minutes); err != nil {
			s.d.fail(w, 400, err)
			return
		}
		if err := s.startLocalAwake(minutes, time.Now()); err != nil {
			s.d.fail(w, 409, err)
			return
		}
		writeJSON(w, 200, "保持唤醒已开始")
	})
	mux.HandleFunc("POST /awake/stop", func(w http.ResponseWriter, r *http.Request) {
		s.sleep.manualUntil = time.Time{}
		s.sleep.autoExpired = true
		if err := s.releaseLocalSleep(); err != nil {
			s.d.fail(w, 503, err)
			return
		}
		writeJSON(w, 200, "已恢复正常睡眠")
	})
}
