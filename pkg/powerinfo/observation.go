// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package powerinfo

import "time"

// Observation is a local, read-only schema. A missing measurement is null.
type Observation struct {
	SchemaVersion int             `json:"schema_version"`
	ObservedAt    time.Time       `json:"observed_at"`
	ReadOnly      bool            `json:"read_only"`
	Source        string          `json:"source"`
	Battery       ObservedBattery `json:"battery"`
	Power         ObservedPower   `json:"power"`
}

type ObservedBattery struct {
	Installed             *bool    `json:"installed"`
	ExternalConnected     *bool    `json:"external_connected"`
	Charging              *bool    `json:"charging"`
	FullyCharged          *bool    `json:"fully_charged"`
	ReportedChargePercent *int64   `json:"reported_charge_percent"`
	RawCurrentCapacityMAh *int64   `json:"raw_current_capacity_mah"`
	RawMaxCapacityMAh     *int64   `json:"raw_max_capacity_mah"`
	DesignCapacityMAh     *int64   `json:"design_capacity_mah"`
	CycleCount            *int64   `json:"cycle_count"`
	VoltageMV             *int64   `json:"voltage_mv"`
	CurrentMA             *int64   `json:"current_ma"`
	TemperatureRaw        *int64   `json:"temperature_raw_uncalibrated"`
	RawCapacityHealth     *float64 `json:"raw_max_over_design_percent"`
}

type ObservedPower struct {
	BatteryWatts      *float64 `json:"battery_watts"`
	AdapterWatts      *float64 `json:"adapter_watts"`
	SystemWatts       *float64 `json:"system_watts"`
	AdapterVoltage    *float64 `json:"adapter_voltage_v"`
	AdapterCurrent    *float64 `json:"adapter_current_a"`
	AdapterRatedWatts *int64   `json:"adapter_rated_watts"`
}

// ReadObservation only uses IORegistry property reads. It does not open SMC,
// load charging configuration, restore a session, or change power settings.
func ReadObservation() (Observation, error) {
	n, f, err := readObservationValues()
	if err != nil {
		return Observation{}, err
	}
	if n[3] == nil || *n[3] <= 0 || n[4] == nil || *n[4] <= 0 {
		if c, err := ReadBatteryDataCapacities(); err == nil {
			if (n[3] == nil || *n[3] <= 0) && c.Max > 0 {
				v := int64(c.Max)
				n[3] = &v
			}
			if (n[4] == nil || *n[4] <= 0) && c.Design > 0 {
				v := int64(c.Design)
				n[4] = &v
			}
		}
	}
	for _, i := range []int{3, 4, 6} {
		if n[i] != nil && *n[i] <= 0 {
			n[i] = nil
		}
	}
	b := ObservedBattery{
		Installed: f[0], ExternalConnected: f[1], Charging: f[2], FullyCharged: f[3],
		RawCurrentCapacityMAh: n[2], RawMaxCapacityMAh: n[3], DesignCapacityMAh: n[4],
		CycleCount: n[5], VoltageMV: n[6], CurrentMA: n[7], TemperatureRaw: n[8],
	}
	if n[0] != nil && n[1] != nil && *n[1] == 100 && *n[0] >= 0 && *n[0] <= 100 {
		b.ReportedChargePercent = n[0]
	}
	if n[3] != nil && *n[3] > 0 && n[4] != nil && *n[4] > 0 {
		v := 100 * float64(*n[3]) / float64(*n[4])
		b.RawCapacityHealth = &v
	}
	var p ObservedPower
	if f[1] != nil && *f[1] {
		if n[9] != nil && *n[9] > 0 {
			v := float64(*n[9]) / 1000
			p.AdapterVoltage = &v
		}
		if n[10] != nil && *n[10] >= 0 {
			v := float64(*n[10]) / 1000
			p.AdapterCurrent = &v
		}
		p.AdapterRatedWatts = readAdapterRatedWatts()
	}
	if n[6] != nil && *n[6] > 0 && n[7] != nil {
		v := float64(*n[6]) * float64(*n[7]) / 1e6
		p.BatteryWatts = &v
	}
	if f[1] != nil && !*f[1] {
		v := 0.0 // No adapter is connected; this is known zero, not missing data.
		p.AdapterWatts = &v
	} else if f[1] != nil && *f[1] && n[9] != nil && *n[9] > 0 && n[10] != nil && *n[10] >= 0 {
		v := float64(*n[9]) * float64(*n[10]) / 1e6
		p.AdapterWatts = &v
	}
	if p.AdapterWatts != nil && p.BatteryWatts != nil {
		v := *p.AdapterWatts - *p.BatteryWatts
		if v >= 0 {
			p.SystemWatts = &v
		}
	}
	return Observation{
		SchemaVersion: 1, ObservedAt: time.Now().UTC(), ReadOnly: true,
		Source: "IORegistry/AppleSmartBattery", Battery: b, Power: p,
	}, nil
}
