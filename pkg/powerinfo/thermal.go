// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package powerinfo

/*
#cgo LDFLAGS: -framework Foundation
int neroSystemThermalState(void);
*/
import "C"

// SystemThermalState is NSProcessInfo's documented thermal-pressure category.
// It is separate from a battery sensor temperature and must not become °C.
func SystemThermalState() int { return int(C.neroSystemThermalState()) }
