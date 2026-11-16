// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package powerinfo

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>
static int neroLidClosed(void) {
 io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching("IOPMrootDomain"));
 if (!service) return -1;
 CFTypeRef value = IORegistryEntryCreateCFProperty(service, CFSTR("AppleClamshellState"), kCFAllocatorDefault, 0);
 IOObjectRelease(service);
 if (!value) return -1;
 int result = CFGetTypeID(value) == CFBooleanGetTypeID() ? (CFBooleanGetValue((CFBooleanRef)value) ? 1 : 0) : -1;
 CFRelease(value);
 return result;
}
*/
import "C"
import "errors"

func LidClosed() (bool, error) {
	v := int(C.neroLidClosed())
	if v < 0 {
		return false, errors.New("cannot verify lid state")
	}
	return v == 1, nil
}
