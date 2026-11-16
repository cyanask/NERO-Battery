// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import <Foundation/Foundation.h>
int neroSystemThermalState(void) {
    @autoreleasepool { return (int)NSProcessInfo.processInfo.thermalState; }
}
