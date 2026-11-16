// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/charlie0129/batt/pkg/powerinfo"
)

func TestPhaseOneLiveObservationWhitelist(t *testing.T) {
	snapshot, err := powerinfo.ReadObservation()
	if err != nil {
		if errors.Is(err, powerinfo.ErrNoSmartBattery) {
			t.Skip("no battery on this machine")
		}
		t.Fatal(err)
	}
	if !snapshot.ReadOnly || snapshot.SchemaVersion != 1 || snapshot.Source != "IORegistry/AppleSmartBattery" || snapshot.ObservedAt.IsZero() {
		t.Fatal("missing read-only observation contract")
	}
	if p := snapshot.Battery.ReportedChargePercent; p != nil && (*p < 0 || *p > 100) {
		t.Fatal("invalid reported battery percentage")
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"schema_version": true, "observed_at": true, "read_only": true, "source": true, "battery": true, "power": true}
	for field := range fields {
		if !allowed[field] {
			t.Fatalf("unexpected field %s in local observation", field)
		}
	}
	for _, section := range []string{"battery", "power"} {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(fields[section], &properties); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"serial", "SerialNumber", "uuid", "hardware_uuid", "email", "ip_address", "files", "processes"} {
			if _, found := properties[forbidden]; found {
				t.Fatalf("private field %s was exposed", forbidden)
			}
		}
	}
}

func TestFillMissingCapacities(t *testing.T) {
	// The BatteryData capacities of a MacBook Pro on macOS 27.2.
	macOS27 := batteryCapacities{Design: 6075, Max: 5752}

	tests := []struct {
		name string
		// top is what the top level of AppleSmartBattery reports.
		top smartBattery
		// read is what its BatteryData dictionary holds.
		read    batteryCapacities
		readErr error

		wantRead   bool
		wantDesign int
		wantMax    int
	}{
		{
			name:     "macOS 27 moved both capacities into BatteryData",
			read:     macOS27,
			wantRead: true, wantDesign: 6075, wantMax: 5752,
		},
		{
			name: "earlier macOS reports both at the top level",
			top:  smartBattery{DesignCapacity: 8694, MaxCapacity: 8573},
			read: macOS27,
			// BatteryData is not consulted, so nothing changes.
			wantDesign: 8694, wantMax: 8573,
		},
		{
			name:     "only a missing capacity is filled in",
			top:      smartBattery{DesignCapacity: 6000},
			read:     macOS27,
			wantRead: true, wantDesign: 6000, wantMax: 5752,
		},
		{
			name:     "BatteryData without the capacities",
			wantRead: true,
		},
		{
			name:     "BatteryData unavailable",
			read:     macOS27,
			readErr:  errors.New("AppleSmartBattery has no BatteryData dictionary"),
			wantRead: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.top
			read := false
			fillMissingCapacities(&b, func() (batteryCapacities, error) {
				read = true
				return tt.read, tt.readErr
			})

			if read != tt.wantRead {
				t.Errorf("BatteryData read = %v, want %v", read, tt.wantRead)
			}
			if b.DesignCapacity != tt.wantDesign || b.MaxCapacity != tt.wantMax {
				t.Errorf("capacities = %d/%d mAh, want %d/%d mAh", b.MaxCapacity, b.DesignCapacity, tt.wantMax, tt.wantDesign)
			}
		})
	}
}

func TestSmartBatterySnapshot(t *testing.T) {
	// A MacBook Pro on macOS 27.2 on a 30 W adapter, discharging toward its
	// charge limit.
	got := smartBattery{
		Voltage:         12260,
		Amperage:        -284,
		CycleCount:      67,
		DesignCapacity:  6075,
		MaxCapacity:     5752,
		SystemVoltageIn: 20152,
		SystemCurrentIn: 30,
	}.snapshot()
	want := BatterySnapshot{
		Voltage:        12.26,
		Amperage:       -0.284,
		CycleCount:     67,
		DesignCapacity: 6075,
		MaxCapacity:    5752,
		InputVoltage:   20.152,
		InputAmperage:  0.03,
	}
	if got != want {
		t.Fatalf("snapshot() = %+v, want %+v", got, want)
	}

	got = smartBattery{IsCharging: true, SystemVoltageIn: -1, SystemCurrentIn: -24}.snapshot()
	if !got.Charging || got.InputVoltage != 0 || got.InputAmperage != 0 {
		t.Fatalf("snapshot() = %+v, want charging with negative adapter readings ignored", got)
	}
}

// TestDarwinBatteryInfoReportsCapacities reads the battery of the Mac running
// the test. It skips on Macs without one, such as CI runners.
func TestDarwinBatteryInfoReportsCapacities(t *testing.T) {
	if c, err := readBatteryDataCapacities(); err != nil || c.Design <= 0 {
		t.Skipf("no battery data on this Mac (design capacity %d mAh, error %v)", c.Design, err)
	}

	b, err := darwinBatteryInfo{}.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if b.DesignCapacity <= 0 || b.MaxCapacity <= 0 || b.Health() <= 0 || b.Voltage <= 0 {
		t.Fatalf("design capacity %d mAh, full-charge capacity %d mAh, health %d%%, voltage %.3f V; want all positive",
			b.DesignCapacity, b.MaxCapacity, b.Health(), b.Voltage)
	}
	t.Logf("%+v, health %d%%", b, b.Health())
}
