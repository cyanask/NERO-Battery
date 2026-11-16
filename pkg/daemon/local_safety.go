// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"errors"
	"math"
	"sync/atomic"
	"time"

	"github.com/charlie0129/batt/pkg/calibration"
	"github.com/charlie0129/batt/pkg/powerinfo"
)

var errLocalThermalPause = errors.New("thermal protection has paused charging and discharging")

type localSafety struct {
	Temperature    *float64 `json:"temperature_c"`
	Sensor         string   `json:"sensor"`
	SystemPressure int      `json:"system_pressure"`
	Paused         bool     `json:"paused"`
	Reason         string   `json:"reason"`
	blocked        atomic.Bool
	enabled        atomic.Bool
	sampledUnix    atomic.Int64
	sleepBlocked   atomic.Bool
	SampledAt      time.Time `json:"sampled_at"`
	Valid          bool      `json:"valid"`
}

// Only known SMC temperature keys decoded using their declared numeric type
// are expressed in Celsius. The old raw IORegistry value stays uncalibrated.
func readLocalBatteryTemperature(s SMC) (*float64, string) {
	var maximum *float64
	source := ""
	for _, key := range []string{"TB0T", "TB1T", "TB2T", "TBAT"} {
		v, err := s.Read(key)
		if err != nil {
			continue
		}
		f, err := v.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 || f > 80 {
			continue
		}
		if maximum == nil || f > *maximum {
			n := f
			maximum = &n
			source = key
		}
	}
	return maximum, source
}

func (s *localSafety) decide(p localThermalSettings, temp *float64, pressure int) {
	s.SampledAt = time.Now()
	s.Valid = temp != nil
	s.enabled.Store(p.Enabled)
	if temp != nil {
		s.sampledUnix.Store(s.SampledAt.UnixNano())
	} else {
		s.sampledUnix.Store(0)
	}
	s.Temperature = temp
	s.SystemPressure = pressure
	if !p.Enabled {
		s.Paused = false
		s.Reason = "热保护已关闭"
		s.blocked.Store(false)
		return
	}
	previous := s.Paused
	switch {
	case temp == nil:
		s.Paused = true
		s.Reason = "电池温度不可用，已暂停受控充电与放电"
	case pressure >= 2:
		s.Paused = true
		s.Reason = "系统热压力较高，已暂停充电与放电"
	case *temp >= p.PauseC:
		s.Paused = true
		s.Reason = "电池达到温度上限，等待降温"
	case previous && *temp > p.ResumeC:
		s.Paused = true
		s.Reason = "等待电池降至恢复温度"
	default:
		s.Paused = false
		s.Reason = "温度正常"
	}
	s.blocked.Store(s.Paused)
}

// Protect every charge-enable and adapter-disable call, including calibration
// transitions. Read-only telemetry, adapter restore and handback remain usable.
type localSafeSMC struct {
	SMC
	safety     *localSafety
	readCharge func() (int, error)
}

func (s localSafeSMC) EnableCharging() error {
	if err := s.safety.controlError(); err != nil {
		return err
	}
	if s.safety.blocked.Load() {
		return errLocalThermalPause
	}
	return s.SMC.EnableCharging()
}
func (s localSafeSMC) DisableAdapter() error {
	if err := s.safety.controlError(); err != nil {
		return err
	}
	if s.safety.blocked.Load() {
		return errLocalThermalPause
	}
	return s.SMC.DisableAdapter()
}

func (s *localService) inspectSafety() error {
	temp, sensor := readLocalBatteryTemperature(s.rawSMC)
	s.safety.Sensor = sensor
	s.safety.decide(s.preferences.Thermal, temp, powerinfo.SystemThermalState())
	if !s.safety.Paused {
		return nil
	}
	d := s.d
	if d.calibrationOwnsChargeLimit() {
		_ = d.pauseCalibration()
	}
	if !d.conf.AdapterDisableUntil().IsZero() {
		if err := d.stopLocalDischarge(); err != nil {
			return err
		}
	}
	if err := d.smc.EnableAdapter(); err != nil {
		return err
	}
	return d.smc.DisableCharging()
}

func (s *localService) onSleep() {
	if s.d.calibrationOwnsChargeLimit() {
		_ = s.d.pauseCalibration()
		_ = s.d.smc.EnableAdapter()
		_ = s.d.smc.DisableCharging()
	}
}

func (s *localService) calibrationBlocked() bool {
	d := s.d
	d.calibrationMu.Lock()
	defer d.calibrationMu.Unlock()
	st := d.calibrationState
	if st.Phase == calibration.PhaseIdle {
		return false
	}
	if !st.Paused && st.Phase != calibration.PhaseError && !st.StartedAt.IsZero() && time.Since(st.StartedAt) > 24*time.Hour {
		st.Paused = true
		st.PauseStartedAt = time.Now()
		st.LastError = "校准超过 24 小时，已安全暂停，请检查后取消或继续"
		_ = d.persistCalibrationState()
	}
	return st.Paused || st.Phase == calibration.PhaseError
}

func (s localSafeSMC) GetBatteryCharge() (int, error) {
	if s.readCharge == nil {
		return 0, errors.New("macOS charge reader unavailable")
	}
	return s.readCharge()
}

var errLocalStaleSafety = errors.New("safety data unavailable or stale")
var errLocalSleepPause = errors.New("sleep protection has paused control")

func (s *localSafety) controlError() error {
	if s.sleepBlocked.Load() {
		return errLocalSleepPause
	}
	if s.enabled.Load() {
		age := time.Now().UnixNano() - s.sampledUnix.Load()
		if s.sampledUnix.Load() == 0 || age < 0 || age > int64(15*time.Second) {
			return errLocalStaleSafety
		}
	}
	return nil
}
func (s *localSafety) invalidate() {
	s.sampledUnix.Store(0)
	s.Valid = false
	s.Temperature = nil
	s.Reason = "保护数据待刷新"
}
