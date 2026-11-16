// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package powerinfo

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>
static long long neroAdapterWatts(void) {
 io_service_t service=IOServiceGetMatchingService(kIOMainPortDefault,IOServiceMatching("AppleSmartBattery"));
 if(!service)return 0;
 CFTypeRef details=IORegistryEntryCreateCFProperty(service,CFSTR("AdapterDetails"),kCFAllocatorDefault,0);
 IOObjectRelease(service);
 long long watts=0;
 if(details&&CFGetTypeID(details)==CFDictionaryGetTypeID()){
  CFTypeRef value=CFDictionaryGetValue((CFDictionaryRef)details,CFSTR("Watts"));
  if(value&&CFGetTypeID(value)==CFNumberGetTypeID())CFNumberGetValue((CFNumberRef)value,kCFNumberLongLongType,&watts);
 }
 if(details)CFRelease(details);
 return watts>0&&watts<=300?watts:0;
}
*/
import "C"

func readAdapterRatedWatts() *int64 {
	v := int64(C.neroAdapterWatts())
	if v == 0 {
		return nil
	}
	return &v
}
