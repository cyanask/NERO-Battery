// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// Calendar-based scheduling, matching the Foundation path observed in AlDente.
#import <Foundation/Foundation.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
int64_t neroNextCalendar(double after,int hour,int minute,int weekdays,const char *zone){@autoreleasepool{
 NSCalendar *calendar=NSCalendar.currentCalendar;calendar.timeZone=(zone&&zone[0])?[NSTimeZone timeZoneWithName:@(zone)]:NSTimeZone.localTimeZone;NSDate *start=[NSDate dateWithTimeIntervalSince1970:after];NSDate *earliest=nil;
 for(int day=0;day<7;day++)if(weekdays&(1<<day)){NSDateComponents *c=NSDateComponents.new;c.hour=hour;c.minute=minute;c.second=0;c.weekday=day+1;NSDate *next=[calendar nextDateAfterDate:start matchingComponents:c options:NSCalendarMatchNextTime|NSCalendarMatchFirst];if(next&&(!earliest||[next compare:earliest]==NSOrderedAscending))earliest=next;}
 return earliest?(int64_t)earliest.timeIntervalSince1970:0;
}}

void neroWaitCalendarTick(void){@autoreleasepool{
 __block BOOL fired=NO;NSTimer *timer=[NSTimer scheduledTimerWithTimeInterval:3 repeats:NO block:^(NSTimer *t){fired=YES;}];
 while(!fired)[NSRunLoop.currentRunLoop runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:4]];
 [timer invalidate];
}}

char *neroCalendarZone(void){@autoreleasepool{[NSTimeZone resetSystemTimeZone];NSTimeZone *z=NSTimeZone.localTimeZone;return strdup([[NSString stringWithFormat:@"%@:%ld",z.name,(long)[z secondsFromGMTForDate:NSDate.date]]UTF8String]);}}

int neroCalendarSameDay(double a,double b,const char *zone){@autoreleasepool{NSCalendar *c=NSCalendar.currentCalendar;c.timeZone=(zone&&zone[0])?[NSTimeZone timeZoneWithName:@(zone)]:NSTimeZone.localTimeZone;return [c isDate:[NSDate dateWithTimeIntervalSince1970:a] inSameDayAsDate:[NSDate dateWithTimeIntervalSince1970:b]]?1:0;}}
