// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func (s *localService) refreshRules(now time.Time) {
	s.nextRules = map[string]time.Time{}
	for _, r := range s.preferences.Schedules {
		if !r.Enabled {
			continue
		}
		spec, err := r.spec()
		if err == nil {
			if r.OnceAt != nil {
				if r.LastRun.IsZero() {
					s.nextRules[r.ID] = *r.OnceAt
				}
			} else {
				next := nextRuleOccurrence(spec, r, now)
				if next.After(now) {
					s.nextRules[r.ID] = next
				}
			}
		}
	}
}

type localActionResult struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *localActionResult) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}
func (w *localActionResult) WriteHeader(code int) { w.code = code }
func (w *localActionResult) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.body.Write(b)
}

func (s *localService) scheduledAction(rule localSchedule) error {
	method, path, body := "POST", "", ""
	switch rule.Action {
	case "limit":
		method, path, body = "PUT", "/limit", fmt.Sprint(rule.Limit)
	case "full":
		path = "/charge-once/full"
	case "cancel-full":
		path = "/charge-once/cancel"
	case "stop-discharge":
		path = "/discharge/stop"
	case "discharge":
		path = "/discharge/start"
	case "power-automatic":
		return setLocalPowerMode("automatic")
	case "power-low":
		return setLocalPowerMode("low-power")
	case "awake-30":
		return s.startLocalAwake(30, time.Now())
	case "awake-60":
		return s.startLocalAwake(60, time.Now())
	case "awake-180":
		return s.startLocalAwake(180, time.Now())
	case "sleep":
		s.sleep.manualUntil = time.Time{}
		s.sleep.autoExpired = true
		return s.releaseLocalSleep()
	case "calibrate":
		return s.startLocalCalibration(20, 60)
	default:
		return errors.New("unknown schedule action")
	}
	if s.safety.Paused && (rule.Action == "full" || rule.Action == "discharge") {
		return errLocalThermalPause
	}
	r, err := http.NewRequest(method, "http://unix"+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	w := &localActionResult{}
	s.routes(s.d.controlRoutes()).ServeHTTP(w, r)
	if w.code >= 300 {
		return errors.New(strings.TrimSpace(w.body.String()))
	}
	return nil
}

func (s *localService) runRules(now time.Time) {
	for i := range s.preferences.Schedules {
		rule := s.preferences.Schedules[i]
		due, ok := s.nextRules[rule.ID]
		if !ok || !rule.Enabled || now.Before(due) {
			continue
		}
		if !rule.LastRun.IsZero() && !due.After(rule.LastRun) {
			spec, err := rule.spec()
			if err == nil {
				s.nextRules[rule.ID] = nextRuleOccurrence(spec, rule, now)
			}
			s.preferences.Schedules[i].Result = "已跳过：重复触发或系统时间回拨"
			continue
		}
		spec, err := rule.spec()
		if err != nil {
			continue
		}
		if rule.OnceAt != nil {
			delete(s.nextRules, rule.ID)
			rule.Enabled = false
		} else {
			s.nextRules[rule.ID] = nextRuleOccurrence(spec, rule, now)
		}
		// Persist admission before executing. Restart never replays old jobs.
		rule.LastRun = now
		rule.Result = "正在执行"
		s.preferences.Schedules[i] = rule
		if err := writePrivateJSON(filepath.Join(s.dir, "preferences.json"), s.preferences); err != nil {
			s.lastError = err.Error()
			rule.Result = "未执行：本机任务状态保存失败"
			s.preferences.Schedules[i] = rule
			continue
		}
		if s.d.localSleeping || now.Sub(due) > 2*time.Minute {
			rule.Result = "已跳过：电脑睡眠或错过执行时间"
		} else if err := s.scheduledAction(rule); err != nil {
			rule.Result = "未执行：" + err.Error()
		} else {
			rule.Result = "已完成"
		}
		s.preferences.Schedules[i] = rule
		if err := writePrivateJSON(filepath.Join(s.dir, "preferences.json"), s.preferences); err != nil {
			s.lastError = err.Error()
		}
	}
}

func (s *localService) reschedule(now time.Time, reason string) {
	changed := false
	for i, rule := range s.preferences.Schedules {
		if !rule.Enabled {
			continue
		}
		if due, ok := s.nextRules[rule.ID]; ok && !due.After(now) {
			rule.LastRun = now
			rule.Result = "已跳过：" + reason
			if rule.OnceAt != nil {
				rule.Enabled = false
			}
		} else {
			rule.Result = "已重新排期：" + reason
		}
		s.preferences.Schedules[i] = rule
		changed = true
	}
	if changed {
		if err := writePrivateJSON(filepath.Join(s.dir, "preferences.json"), s.preferences); err != nil {
			s.lastError = "重新排期记录未保存：" + err.Error()
		}
	}
	s.refreshRules(now)
	s.clockLast = now
	s.clockZone = localCalendarZone()
}
func (s *localService) checkClock(now time.Time) {
	zone := localCalendarZone()
	if s.clockLast.IsZero() {
		s.clockLast = now
		s.clockZone = zone
		return
	}
	wall := now.UnixNano() - s.clockLast.UnixNano()
	elapsed := int64(now.Sub(s.clockLast))
	jump := wall - elapsed
	if zone != s.clockZone || jump > int64(2*time.Second) || jump < -int64(2*time.Second) {
		s.reschedule(now, "系统时间或时区变化")
	}
	s.clockLast = now
	s.clockZone = zone
}
