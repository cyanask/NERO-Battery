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

NSDictionary *NeroSMCUSBPowerSnapshot(void);

// Fallback only. These system-reported values may refresh more slowly than SMC.
// Never substitute PD offers, budgets or input/battery differences.
static NSDictionary *systemUSBPowerSnapshot(void) {
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
 return @{@"schema":@2,@"source":@"IORegistry/AppleSmartBattery/PowerOutDetails",@"state":state,@"read_only":@YES,@"reading_kind":@"system_report",@"usb_c_port_numbers":inventory,@"ports":ports,@"reported_total_watts":complete?@(total):NSNull.null,@"total_watts":complete?@(total):NSNull.null,@"total_scope":@"reported_entries_only",@"device_attribution":@"unavailable"};
}

NSDictionary *NeroUSBPowerSnapshot(void){
 NSDictionary *smc=NeroSMCUSBPowerSnapshot();
 // Partial SMC results retain unknown totals. Never combine unlike providers.
 if([smc[@"valid_port_count"]unsignedIntegerValue]>0)return smc;
 NSMutableDictionary *fallback=[systemUSBPowerSnapshot()mutableCopy];
 fallback[@"fallback_reason"]=@"SMC readings unavailable or port mapping unverified";
 fallback[@"smc_state"]=smc[@"state"];
 return fallback;
}

char *neroUSBPowerJSON(void){@autoreleasepool{
 NSData *data=[NSJSONSerialization dataWithJSONObject:NeroUSBPowerSnapshot() options:NSJSONWritingSortedKeys error:nil];
 return strdup(data?[[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding]UTF8String]:"{}");
}}

void NeroRenderUSBPower(NeroController *controller){
 NSDictionary *s=NeroUSBPowerSnapshot();
 BOOL smc=[s[@"reading_kind"]isEqual:@"voltage_times_current"];
 NeroValue(controller,@"usb-output",NeroMetricString(s[@"total_watts"],@" W",2));
 NSMutableArray *lines=NSMutableArray.new;
 for(NSDictionary *port in s[@"ports"]){
  NSNumber *index=usbNumber(port[smc?@"system_port_number":@"system_port_index"]);
  NSString *name=index?[NSString stringWithFormat:smc?@"USB-C %@":@"系统输出端口 %@",index]:[NSString stringWithFormat:@"报告项 %@",port[@"report_index"]];
  NSString *electrical=smc&&[port[@"device_reply_verified"]boolValue]?[NSString stringWithFormat:@"（%@ × %@）",NeroMetricString(port[@"volts"],@" V",3),NeroMetricString(port[@"amps"],@" A",3)]:@"";
  [lines addObject:[NSString stringWithFormat:@"%@  ·  %@%@",name,NeroMetricString(port[@"watts"],@" W",2),electrical]];
 }
 if(!lines.count)for(NSNumber *number in s[@"usb_c_port_numbers"])[lines addObject:[NSString stringWithFormat:@"USB-C %@  ·  未报告",number]];
 NeroValue(controller,@"usb-ports",lines.count?[lines componentsJoinedByString:@"     "]:@"端口功率未报告");
 NSString *note=smc?([s[@"state"]isEqual:@"reported"]?@"SMC · 电压 × 电流换算；合计仅含已匹配的 USB-C 端口，不含 MagSafe。":@"SMC · 电压 × 电流换算；部分端口不可用，合计保持未知，不补零。"):
 ([s[@"state"]isEqual:@"reported"]?@"SMC 暂不可用，现为系统报告；刷新可能较慢，系统索引不等于具体设备。":@"输出读数暂不可用；未报告不代表 0 W。不使用协商功率或申报电流。 ");
 NeroValue(controller,@"usb-note",note);
}
