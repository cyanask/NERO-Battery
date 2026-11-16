// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

/*
#cgo LDFLAGS:  -framework CoreFoundation -framework IOKit

#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/pwr_mgt/IOPMLib.h>

static IOReturn neroCreateLocalAssertion(IOPMAssertionID *id){return IOPMAssertionCreateWithName(kIOPMAssertPreventUserIdleSystemSleep,kIOPMAssertionLevelOn,CFSTR("Battery awake session"),id);}
// Expose the macro
const CFStringRef AssertionTypePreventSystemSleep = kIOPMAssertionTypePreventSystemSleep;
const CFStringRef AssertionTypePreventUserIdleSystemSleep = kIOPMAssertPreventUserIdleSystemSleep;
const IOPMAssertionID NullAssertionID = kIOPMNullAssertionID;
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

func createSleepAssertion(assertionType C.CFStringRef, name, details string) (C.IOPMAssertionID, error) {
	cname := C.CString(name)
	cdetail := C.CString(details)
	defer C.free(unsafe.Pointer(cname))
	defer C.free(unsafe.Pointer(cdetail))

	cfName := C.CFStringCreateWithCString(
		C.kCFAllocatorDefault,
		cname,
		C.kCFStringEncodingUTF8,
	)
	cfDetails := C.CFStringCreateWithCString(
		C.kCFAllocatorDefault,
		cdetail,
		C.kCFStringEncodingUTF8,
	)
	defer C.CFRelease(C.CFTypeRef(cfName))
	defer C.CFRelease(C.CFTypeRef(cfDetails))

	var assertionID C.IOPMAssertionID
	status := C.IOPMAssertionCreateWithDescription(
		assertionType,
		cfName,
		cfDetails,
		0,
		0,
		0,
		0,
		&assertionID,
	)
	if status != C.kIOReturnSuccess {
		return 0, fmt.Errorf("IOPMAssertionCreateWithDescription failed: 0x%x", uint32(status))
	}
	return assertionID, nil
}

func releaseAssertion(assertionID C.IOPMAssertionID) error {
	status := C.IOPMAssertionRelease(assertionID)
	if status != C.kIOReturnSuccess {
		return fmt.Errorf("IOPMAssertionRelease failed: 0x%x", uint32(status))
	}
	return nil
}

// iokitAssertions holds the daemon's IOPM power assertions. The zero value
// holds none: kIOPMNullAssertionID is 0.
type iokitAssertions struct {
	mu          sync.Mutex
	local       C.IOPMAssertionID
	onAC        C.IOPMAssertionID
	calibration C.IOPMAssertionID
}

// PreventSleepOnAC keeps the system in dark wake. It has an effect on AC only.
func (a *iokitAssertions) PreventSleepOnAC() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.onAC != C.NullAssertionID {
		return nil
	}
	id, err := createSleepAssertion(C.AssertionTypePreventSystemSleep, "batt", "Maintained charging by batt is in progress")
	if err != nil {
		return err
	}
	a.onAC = id
	return nil
}

// AllowSleepOnAC releases the dark-wake assertion, allowing sleep on AC.
func (a *iokitAssertions) AllowSleepOnAC() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.onAC == C.NullAssertionID {
		return nil
	}
	err := releaseAssertion(a.onAC)
	a.onAC = C.NullAssertionID
	return err
}

// PreventCalibrationSleep prevents idle system sleep on both battery and AC
// power. macOS still permits forced sleep, such as closing the lid.
func (a *iokitAssertions) PreventCalibrationSleep() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calibration != C.NullAssertionID {
		return nil
	}
	id, err := createSleepAssertion(
		C.AssertionTypePreventUserIdleSystemSleep,
		"batt auto calibration",
		"Auto calibration is in progress",
	)
	if err != nil {
		return err
	}
	a.calibration = id
	return nil
}

// AllowCalibrationSleep releases the calibration-specific sleep assertion.
func (a *iokitAssertions) AllowCalibrationSleep() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calibration == C.NullAssertionID {
		return nil
	}
	err := releaseAssertion(a.calibration)
	a.calibration = C.NullAssertionID
	return err
}

// Local awake holds are independent of calibration and drop on process exit.
func (a *iokitAssertions) PreventLocalSleep() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.local != C.NullAssertionID {
		return nil
	}
	var id C.IOPMAssertionID
	if status := C.neroCreateLocalAssertion(&id); status != C.kIOReturnSuccess {
		return fmt.Errorf("IOPMAssertionCreateWithName: 0x%x", uint32(status))
	}
	a.local = id
	return nil
}
func (a *iokitAssertions) AllowLocalSleep() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.local == C.NullAssertionID {
		return nil
	}
	err := releaseAssertion(a.local)
	if err == nil {
		a.local = C.NullAssertionID
	}
	return err
}
