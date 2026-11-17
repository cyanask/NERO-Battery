// NERO Battery modifications, 2026-11-17. GPL v2; see docs/CHANGES.md.
#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>
#include <math.h>
#include <stddef.h>
#include <stdio.h>
#include <string.h>

// AppleSMC user-client ABI. This reader exposes no write command.
typedef struct { uint8_t major,minor,build,reserved; uint16_t release; } NeroSMCVersion;
typedef struct { uint16_t version,length; uint32_t cpu,gpu,memory; } NeroSMCLimits;
typedef struct { uint32_t size,type; uint8_t attributes; } NeroSMCInfo;
typedef struct {
 uint32_t key; NeroSMCVersion version; NeroSMCLimits limits; NeroSMCInfo info;
 uint8_t result,status,command; uint32_t data; uint8_t bytes[32];
} NeroSMCData;
_Static_assert(sizeof(NeroSMCData)==80,"SMC ABI size");
_Static_assert(offsetof(NeroSMCData,result)==40,"SMC result offset");
_Static_assert(offsetof(NeroSMCData,bytes)==48,"SMC bytes offset");

static uint32_t smcCode(const char *s){return ((uint32_t)(uint8_t)s[0]<<24)|((uint32_t)(uint8_t)s[1]<<16)|((uint32_t)(uint8_t)s[2]<<8)|(uint8_t)s[3];}
static id smcProperty(io_registry_entry_t e,NSString *key){return CFBridgingRelease(IORegistryEntryCreateCFProperty(e,(__bridge CFStringRef)key,kCFAllocatorDefault,0));}
static NSString *smcUUID(id value){
 if(![value isKindOfClass:NSString.class])return nil;
 NSString *s=[[value stringByReplacingOccurrencesOfString:@"-" withString:@""]lowercaseString];
 if(s.length!=32||[s rangeOfCharacterFromSet:[[NSCharacterSet characterSetWithCharactersInString:@"0123456789abcdef"]invertedSet]].location!=NSNotFound||[s isEqual:@"00000000000000000000000000000000"])return nil;
 return s;
}
static NSArray *smcUSBPorts(void){
 NSMutableArray *ports=NSMutableArray.new;io_iterator_t iterator=0;
 if(IOServiceGetMatchingServices(kIOMainPortDefault,IOServiceMatching("AppleHPMInterface"),&iterator)!=KERN_SUCCESS)return ports;
 io_registry_entry_t e;
 while((e=IOIteratorNext(iterator))){
  id n=smcProperty(e,@"PortNumber");
  if([smcProperty(e,@"PortTypeDescription")isEqual:@"USB-C"]&&[n isKindOfClass:NSNumber.class]&&CFGetTypeID((__bridge CFTypeRef)n)!=CFBooleanGetTypeID()&&isfinite([n doubleValue])&&[n doubleValue]>=0&&[n doubleValue]<=255&&floor([n doubleValue])==[n doubleValue]){
   NSString *uuid=nil;io_registry_entry_t ancestor=e;IOObjectRetain(ancestor);
   for(int depth=0;depth<16;depth++){
    if(IOObjectConformsTo(ancestor,"AppleHPMDevice")){uuid=smcUUID(smcProperty(ancestor,@"UUID"));break;}
    io_registry_entry_t parent=0;
    if(IORegistryEntryGetParentEntry(ancestor,kIOServicePlane,&parent)!=KERN_SUCCESS)break;
    IOObjectRelease(ancestor);ancestor=parent;
   }
   IOObjectRelease(ancestor);
   NSDictionary *port=@{@"number":n,@"uuid":uuid?:NSNull.null};
   if(![ports containsObject:port])[ports addObject:port];
  }
  IOObjectRelease(e);
 }
 BOOL valid=IOIteratorIsValid(iterator);IOObjectRelease(iterator);
 return valid?[ports sortedArrayUsingComparator:^NSComparisonResult(NSDictionary *a,NSDictionary *b){return [a[@"number"]compare:b[@"number"]];}]:@[];
}
static BOOL smcCall(io_connect_t connection,NeroSMCData *input,NeroSMCData *output,NSString **error){
 if(input->command!=5&&input->command!=9){*error=@"read_command_rejected";return NO;}
 memset(output,0,sizeof(*output));size_t size=sizeof(*output);
 kern_return_t kr=IOConnectCallStructMethod(connection,2,input,sizeof(*input),output,&size);
 if(kr!=KERN_SUCCESS||size!=sizeof(*output)||output->result!=0||output->status!=0){
  *error=[NSString stringWithFormat:@"SMC %@: IOKit=0x%x result=0x%02x status=0x%02x size=%zu",input->command==9?@"info":@"read",kr,output->result,output->status,size];return NO;
 }
 return YES;
}
static NSData *smcRead(io_connect_t c,const char *key,const char *type,uint32_t expected,NSString **error){
 NeroSMCData in={0},out={0};in.key=smcCode(key);in.command=9;
 if(!smcCall(c,&in,&out,error))return nil;
 if(out.info.size!=expected||expected>32||out.info.type!=smcCode(type)){*error=@"unexpected_key_type_or_size";return nil;}
 in.info=out.info;in.command=5;
 if(!smcCall(c,&in,&out,error))return nil;
 return [NSData dataWithBytes:out.bytes length:expected];
}
static NSNumber *smcFloat(io_connect_t c,const char *key,double maximum,NSString **error){
 NSData *bytes=smcRead(c,key,"flt ",4,error);if(!bytes)return nil;
 // Apple Silicon flt is IEEE754 little-endian, not fixed-point or a PD budget.
 uint32_t bits=0;memcpy(&bits,bytes.bytes,4);bits=CFSwapInt32LittleToHost(bits);float value;memcpy(&value,&bits,4);
 if(!isfinite(value)||value<0||value>maximum){*error=@"invalid_sensor_value";return nil;}
 return @(value);
}

NSDictionary *NeroSMCUSBPowerSnapshot(void){
 NSArray *inventory=smcUSBPorts();NSMutableArray *ports=NSMutableArray.new;
 io_service_t device=IOServiceGetMatchingService(kIOMainPortDefault,IOServiceMatching("AppleSMC"));io_connect_t connection=0;
 kern_return_t opened=device?IOServiceOpen(device,mach_task_self(),0,&connection):kIOReturnNotFound;
 if(device)IOObjectRelease(device);
 NSMutableDictionary *channels=NSMutableDictionary.new;
 if(opened==KERN_SUCCESS){
  for(int i=1;i<=4;i++){
   char key[5];snprintf(key,sizeof(key),"D%dUI",i);NSString *error=nil;
   NSData *bytes=smcRead(connection,key,"hex_",16,&error);if(!bytes)continue;
   const uint8_t *raw=bytes.bytes;NSMutableString *uuid=NSMutableString.new;
   for(int j=0;j<16;j++)[uuid appendFormat:@"%02x",raw[j]];
   if(!smcUUID(uuid))continue;
   NSMutableArray *matches=channels[uuid];if(!matches){matches=NSMutableArray.new;channels[uuid]=matches;}[matches addObject:@(i)];
  }
 }
 double total=0;NSUInteger valid=0;
 for(NSDictionary *p in inventory){
  NSNumber *number=p[@"number"];NSString *uuid=[p[@"uuid"]isKindOfClass:NSString.class]?p[@"uuid"]:nil;
  NSArray *matches=uuid?channels[uuid]:nil;
  NSUInteger sameNumber=0,sameUUID=0;
  for(NSDictionary *other in inventory){if([other[@"number"]isEqual:number])sameNumber++;if(uuid&&[other[@"uuid"]isEqual:uuid])sameUUID++;}
  NSString *error=opened==KERN_SUCCESS?@"channel_unmapped":@"smc_open_unavailable";
  NSNumber *volts=nil,*amps=nil,*present=nil,*channel=nil;BOOL verified=NO;
  if(matches.count==1&&sameNumber==1&&sameUUID==1){
   channel=matches[0];char key[5];int i=channel.intValue;error=nil;
   snprintf(key,sizeof(key),"D%dJV",i);volts=smcFloat(connection,key,60,&error);
   if(volts){snprintf(key,sizeof(key),"D%dJI",i);amps=smcFloat(connection,key,10,&error);}
   if(amps){snprintf(key,sizeof(key),"D%dPR",i);NSData *flag=smcRead(connection,key,"ui8 ",1,&error);if(flag)present=@(*(const uint8_t *)flag.bytes);}
   verified=volts&&amps&&present;
   if(verified&&((volts.doubleValue==0&&amps.doubleValue>0)||volts.doubleValue*amps.doubleValue>240)){verified=NO;error=@"inconsistent_output_sensors";}
  }
  id watts=verified?@(volts.doubleValue*amps.doubleValue):NSNull.null;
  if(verified){total+=[watts doubleValue];valid++;}
  [ports addObject:@{@"system_port_number":number,@"smc_channel":channel?:NSNull.null,@"volts":verified?volts:NSNull.null,@"amps":verified?amps:NSNull.null,@"presence_flag":verified?present:NSNull.null,@"watts":watts,@"basis":@"SMC.voltage_times_current",@"device_reply_verified":@(verified),@"state":verified?@"reported":@"unavailable",@"error":verified?NSNull.null:(error?:@"unavailable")}];
 }
 if(connection)IOServiceClose(connection);
 BOOL complete=inventory.count>0&&valid==inventory.count;
 return @{@"schema":@2,@"source":@"SMC/DnJV,DnJI",@"reading_kind":@"voltage_times_current",@"state":complete?@"reported":(valid?@"partial":@"unavailable"),@"read_only":@YES,@"ports":ports,@"valid_port_count":@(valid),@"total_watts":complete?@(total):NSNull.null,@"total_scope":@"matched_usb_c_ports_only",@"device_attribution":@"unavailable",@"sampled_at":@([NSDate.date timeIntervalSince1970]),@"device_reply_verified":@(complete)};
}
