// NERO Battery modifications, 2026-11-16. GPL v2; see docs/CHANGES.md.
#import "native.h"
#import <IOKit/IOKitLib.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

static id usbProperty(io_registry_entry_t entry, NSString *key) {
 return CFBridgingRelease(IORegistryEntryCreateCFProperty(entry,(__bridge CFStringRef)key,kCFAllocatorDefault,0));
}
static NSNumber *usbNumber(id value) {
 if(![value isKindOfClass:NSNumber.class]||CFGetTypeID((__bridge CFTypeRef)value)==CFBooleanGetTypeID())return nil;
 return isfinite([value doubleValue])?value:nil;
}

// Only collect power reported in AppleSmartBattery.PowerOutDetails. PD offers,
// requested current, max-current budgets and input/battery differences are never
// substituted for an output measurement. Port indices are opaque system IDs.
NSDictionary *NeroUSBPowerSnapshot(void) {
 NSMutableArray *inventory=NSMutableArray.new,*ports=NSMutableArray.new;
 io_iterator_t iterator=IO_OBJECT_NULL;
 if(IOServiceGetMatchingServices(kIOMainPortDefault,IOServiceMatching("AppleHPMInterface"),&iterator)==KERN_SUCCESS){
  io_registry_entry_t entry;
  while((entry=IOIteratorNext(iterator))){
   if([usbProperty(entry,@"PortTypeDescription") isEqual:@"USB-C"]){
    NSNumber *number=usbNumber(usbProperty(entry,@"PortNumber"));
    if(number&&number.longLongValue>=0&&![inventory containsObject:number])[inventory addObject:number];
   }
   IOObjectRelease(entry);
  }
  IOObjectRelease(iterator);
 }
 [inventory sortUsingSelector:@selector(compare:)];
 io_registry_entry_t battery=IOServiceGetMatchingService(kIOMainPortDefault,IOServiceMatching("AppleSmartBattery"));
 id details=battery?usbProperty(battery,@"PowerOutDetails"):nil;
 if(battery)IOObjectRelease(battery);
 BOOL array=[details isKindOfClass:NSArray.class];
 NSString *state=details?(array?@"empty":@"invalid"):@"not_reported";
 double total=0;BOOL complete=array&&[details count]>0;NSUInteger itemIndex=0;
 if(array&&[details count]<=64){
  for(id raw in details){
   itemIndex++;
   if(![raw isKindOfClass:NSDictionary.class]){complete=NO;continue;}
   NSNumber *index=usbNumber(raw[@"PortIndex"]);
   if(index&&(index.doubleValue<0||index.doubleValue>255||floor(index.doubleValue)!=index.doubleValue))index=nil;
   NSNumber *value=usbNumber(raw[@"Watts"]);
   // AlDente 1.40.1's official binary reads this entry as a Float, divides
   // by 1000 and sums the records. We independently read the same protocol.
   id watts=NSNull.null;
   if(value&&value.doubleValue>=0&&value.doubleValue<=1000000){
    // Keep the system-reported scope; never use the unvalidated PDPowermW
    // fallback or advertise independent calibration/device attribution.
    watts=@(value.doubleValue/1000.0);total+=[watts doubleValue];
   }else complete=NO;
   [ports addObject:@{@"report_index":@(itemIndex),@"system_port_index":index?:NSNull.null,@"watts":watts,@"basis":@"PowerOutDetails.Watts"}];
  }
  if(ports.count)state=complete?@"reported":@"partial";
 }else if(array){complete=NO;state=@"invalid";}
 return @{@"source":@"IORegistry/AppleSmartBattery/PowerOutDetails",@"state":state,@"read_only":@YES,@"reading_kind":@"system_report",@"usb_c_port_numbers":inventory,@"ports":ports,@"reported_total_watts":complete?@(total):NSNull.null,@"total_scope":@"reported_entries_only",@"device_attribution":@"unavailable"};
}

char *neroUSBPowerJSON(void){@autoreleasepool{
 NSData *data=[NSJSONSerialization dataWithJSONObject:NeroUSBPowerSnapshot() options:NSJSONWritingSortedKeys error:nil];
 return strdup(data?[[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding]UTF8String]:"{}");
}}

void NeroRenderUSBPower(NeroController *controller){
 NSDictionary *s=NeroUSBPowerSnapshot();
 NeroValue(controller,@"usb-output",NeroMetricString(s[@"reported_total_watts"],@" W",2));
 NSMutableArray *lines=NSMutableArray.new;
 for(NSDictionary *port in s[@"ports"]){
  NSNumber *index=usbNumber(port[@"system_port_index"]);
  NSString *name=index?[NSString stringWithFormat:@"系统输出端口 %@",index]:[NSString stringWithFormat:@"报告项 %@",port[@"report_index"]];
  [lines addObject:[NSString stringWithFormat:@"%@  ·  %@",name,NeroMetricString(port[@"watts"],@" W",2)]];
 }
 if(!lines.count)for(NSNumber *number in s[@"usb_c_port_numbers"])[lines addObject:[NSString stringWithFormat:@"USB-C %@  ·  未报告",number]];
 NeroValue(controller,@"usb-ports",lines.count?[lines componentsJoinedByString:@"     "]:@"端口功率未报告");
 NSString *note=[s[@"state"] isEqual:@"reported"]?@"仅合计系统报告的输出记录；系统端口索引尚未映射到具体设备。":@"系统未提供完整输出读数；未报告不代表 0 W。协商功率和设备申报电流不计入。";
 NeroValue(controller,@"usb-note",note);
}
