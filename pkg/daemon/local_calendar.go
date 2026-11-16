// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdint.h>
#include <stdlib.h>
char *neroCalendarZone(void);
int neroCalendarSameDay(double a,double b,const char *zone);
void neroWaitCalendarTick(void);
int64_t neroNextCalendar(double after, int hour, int minute, int weekdays,const char *zone);
*/
import "C"
import "time"
import "unsafe"
import "context"

type localCalendarSchedule struct {
	hour, minute, weekdays int
	zone                   string
}

func (s localCalendarSchedule) Next(after time.Time) time.Time {
	zone := C.CString(s.zone)
	defer C.free(unsafe.Pointer(zone))
	n := int64(C.neroNextCalendar(C.double(float64(after.UnixNano())/1e9), C.int(s.hour), C.int(s.minute), C.int(s.weekdays), zone))
	if n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

func localCalendarTicks(ctx context.Context) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			C.neroWaitCalendarTick()
			select {
			case ch <- time.Now():
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func localCalendarZone() string {
	v := C.neroCalendarZone()
	defer C.free(unsafe.Pointer(v))
	return C.GoString(v)
}

func (s localCalendarSchedule) sameDay(a, b time.Time) bool {
	z := C.CString(s.zone)
	defer C.free(unsafe.Pointer(z))
	return C.neroCalendarSameDay(C.double(a.Unix()), C.double(b.Unix()), z) != 0
}
func nextRuleOccurrence(spec localCalendarSchedule, rule localSchedule, after time.Time) time.Time {
	cursor := after
	if rule.LastRun.After(cursor) {
		cursor = rule.LastRun
	}
	for i := 0; i < 3; i++ {
		next := spec.Next(cursor)
		if !next.After(cursor) {
			return time.Time{}
		}
		if rule.LastRun.IsZero() || !spec.sameDay(next, rule.LastRun) {
			return next
		}
		cursor = next
	}
	return time.Time{}
}
