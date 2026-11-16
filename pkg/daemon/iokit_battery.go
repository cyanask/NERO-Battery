// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import "github.com/charlie0129/batt/pkg/powerinfo"

// Keep the existing daemon seam while sharing its read-only IOKit reader.
type smartBattery powerinfo.SmartBattery
type batteryCapacities = powerinfo.BatteryCapacities

func readSmartBattery() (smartBattery, error) {
	b, err := powerinfo.ReadSmartBattery()
	return smartBattery(b), err
}

func readBatteryDataCapacities() (batteryCapacities, error) {
	return powerinfo.ReadBatteryDataCapacities()
}
