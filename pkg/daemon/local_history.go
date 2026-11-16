// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/charlie0129/batt/pkg/powerinfo"
)

const historyRetentionDays = 30
const historyDailyMaxBytes = 2 * 1024 * 1024

type historyPoint struct {
	At          time.Time `json:"at"`
	Charge      *int64    `json:"charge"`
	Temperature *float64  `json:"temperature_c"`
	BatteryW    *float64  `json:"battery_w"`
	AdapterW    *float64  `json:"adapter_w"`
	SystemW     *float64  `json:"system_w"`
	Limit       int       `json:"limit"`
	BreakBefore bool      `json:"break_before,omitempty"`
}

type localHistory struct {
	prunedDay string
	dir       string
	next      time.Time
	LastError string
	LastAt    time.Time
}

func (h *localHistory) record(now time.Time, o powerinfo.Observation, temp *float64, limit int) {
	if now.Before(h.next) {
		return
	}
	h.next = now.Add(30 * time.Second)
	h.LastError = ""
	if err := os.MkdirAll(h.dir, 0700); err != nil {
		h.LastError = err.Error()
		return
	}
	path := filepath.Join(h.dir, now.UTC().Format("2006-01-02")+".jsonl")
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		h.LastError = err.Error()
		return
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() >= historyDailyMaxBytes {
		h.LastError = "daily history storage bound reached"
		return
	}
	b, err := json.Marshal(historyPoint{At: now.UTC(), Charge: o.Battery.ReportedChargePercent, Temperature: temp, BatteryW: o.Power.BatteryWatts, AdapterW: o.Power.AdapterWatts, SystemW: o.Power.SystemWatts, Limit: limit})
	if err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		h.LastError = err.Error()
		return
	}
	h.LastAt = now.UTC()
	day := now.UTC().Format("2006-01-02")
	if h.prunedDay == day {
		return
	}
	// Only app-owned files with the exact daily filename contract expire.
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return
	}
	h.prunedDay = day
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.jsonl$`)
	cutoff := now.UTC().AddDate(0, 0, -historyRetentionDays).Truncate(24 * time.Hour)
	for _, e := range entries {
		if !e.Type().IsRegular() || !pattern.MatchString(e.Name()) {
			continue
		}
		day, err := time.Parse("2006-01-02.jsonl", e.Name())
		if err == nil && day.Before(cutoff) {
			if err := os.Remove(filepath.Join(h.dir, e.Name())); err != nil {
				h.LastError = err.Error()
			}
		}
	}
}

func (h *localHistory) read(now time.Time, period string) ([]historyPoint, error) {
	durations := map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}
	dur, ok := durations[period]
	if !ok {
		return nil, errors.New("choose 1h, 24h, 7d or 30d")
	}
	points := []historyPoint{}
	cutoff := now.Add(-dur)
	for day := cutoff.UTC().Truncate(24 * time.Hour); !day.After(now); day = day.Add(24 * time.Hour) {
		path := filepath.Join(h.dir, day.Format("2006-01-02")+".jsonl")
		fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		f := os.NewFile(uintptr(fd), path)
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() || st.Size() > historyDailyMaxBytes+4096 {
			f.Close()
			return nil, errors.New("invalid history file")
		}
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 4096), 8192)
		for scan.Scan() {
			var p historyPoint
			if json.Unmarshal(scan.Bytes(), &p) == nil && !p.At.Before(cutoff) && !p.At.After(now) {
				points = append(points, p)
			}
		}
		err = scan.Err()
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	// Preserve a bounded real sample series; never manufacture missing history.
	sort.SliceStable(points, func(i, j int) bool { return points[i].At.Before(points[j].At) })
	for i := 1; i < len(points); i++ {
		points[i].BreakBefore = points[i].At.Sub(points[i-1].At) > 90*time.Second
	}
	if len(points) > 2500 {
		stride := (len(points) + 2499) / 2500
		reduced := make([]historyPoint, 0, 2501)
		previous := -1
		for i := 0; i < len(points); i += stride {
			p := points[i]
			for j := previous + 1; j <= i; j++ {
				if points[j].BreakBefore {
					p.BreakBefore = true
				}
			}
			reduced = append(reduced, p)
			previous = i
		}
		if reduced[len(reduced)-1].At != points[len(points)-1].At {
			p := points[len(points)-1]
			for j := previous + 1; j < len(points); j++ {
				if points[j].BreakBefore {
					p.BreakBefore = true
				}
			}
			reduced = append(reduced, p)
		}
		points = reduced
	}
	return points, nil
}
