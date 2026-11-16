// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type localThermalSettings struct {
	Enabled bool    `json:"enabled"`
	PauseC  float64 `json:"pause_c"`
	ResumeC float64 `json:"resume_c"`
}

type localSchedule struct {
	Name    string     `json:"name,omitempty"`
	OnceAt  *time.Time `json:"once_at,omitempty"`
	NextRun time.Time  `json:"next_run,omitempty"`
	ID      string     `json:"id"`
	Enabled bool       `json:"enabled"`
	Action  string     `json:"action"`
	Hour    int        `json:"hour"`
	Minute  int        `json:"minute"`
	Days    string     `json:"days"`
	Limit   int        `json:"limit"`
	LastRun time.Time  `json:"last_run"`
	Result  string     `json:"result"`
}

type localPreferences struct {
	Sleep     localSleepPolicy     `json:"sleep"`
	Schema    int                  `json:"schema"`
	Thermal   localThermalSettings `json:"thermal"`
	Schedules []localSchedule      `json:"schedules"`
}

func defaultLocalPreferences() localPreferences {
	return localPreferences{Sleep: localSleepPolicy{StopCharging: true, MaxMinutes: 120, Floor: 20}, Schema: 1, Thermal: localThermalSettings{true, 40, 37}, Schedules: []localSchedule{}}
}

func (p localPreferences) validate() error {
	if p.Sleep.MaxMinutes < 1 || p.Sleep.MaxMinutes > 480 || p.Sleep.Floor < 10 || p.Sleep.Floor > 50 {
		return errors.New("invalid sleep safety limits")
	}
	if p.Schema != 1 || p.Thermal.PauseC < 35 || p.Thermal.PauseC > 45 || p.Thermal.ResumeC < 30 || p.Thermal.ResumeC > p.Thermal.PauseC-2 {
		return errors.New("thermal limits require 35–45°C pause and at least 2°C hysteresis")
	}
	if len(p.Schedules) > 16 {
		return errors.New("at most 16 schedules are supported")
	}
	seen := map[string]bool{}
	for _, item := range p.Schedules {
		if item.ID == "" || len(item.ID) > 40 || seen[item.ID] {
			return errors.New("invalid schedule identity")
		}
		seen[item.ID] = true
		if _, err := item.spec(); err != nil {
			return err
		}
	}
	return nil
}

func (s localSchedule) spec() (localCalendarSchedule, error) {
	if s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 {
		return localCalendarSchedule{}, errors.New("invalid schedule time")
	}
	if len(s.Name) > 120 {
		return localCalendarSchedule{}, errors.New("schedule name too long")
	}
	if s.OnceAt != nil && s.OnceAt.IsZero() {
		return localCalendarSchedule{}, errors.New("invalid one-time date")
	}
	if s.Days != "*" && s.Days != "1-5" && s.Days != "0,6" {
		seen := map[int]bool{}
		for _, v := range strings.Split(s.Days, ",") {
			n, e := strconv.Atoi(v)
			if e != nil || n < 0 || n > 6 || seen[n] {
				return localCalendarSchedule{}, errors.New("invalid weekdays")
			}
			seen[n] = true
		}
	}
	switch s.Action {
	case "limit":
		if s.Limit < 50 || s.Limit > 100 {
			return localCalendarSchedule{}, errors.New("schedule limit must be 50–100%")
		}
	case "full", "cancel-full", "stop-discharge", "discharge", "calibrate", "power-automatic", "power-low", "awake-30", "awake-60", "awake-180", "sleep":
	default:
		return localCalendarSchedule{}, errors.New("this schedule action is unavailable")
	}
	mask := 127
	if s.Days == "1-5" {
		mask = 62
	} else if s.Days == "0,6" {
		mask = 65
	} else if s.Days != "*" {
		mask = 0
		for _, part := range strings.Split(s.Days, ",") {
			n, _ := strconv.Atoi(part)
			mask |= 1 << n
		}
	}
	return localCalendarSchedule{hour: s.Hour, minute: s.Minute, weekdays: mask}, nil
}

func loadLocalPreferences(dir string) (localPreferences, error) {
	p := defaultLocalPreferences()
	b, err := os.ReadFile(filepath.Join(dir, "preferences.json"))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(b) > 32*1024 {
		return p, errors.New("local preferences exceed their size bound")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, errors.New("unexpected trailing settings data")
	}
	return p, p.validate()
}

func sameLocalDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
