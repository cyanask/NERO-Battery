// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"
#import <Intents/Intents.h>
#include <string.h>
@interface EOBatteryLevelIntent:INIntent

@end
@implementation EOBatteryLevelIntent

@end
@interface EOBatteryLevelIntentResponse:INIntentResponse
@property(nonatomic,readonly) NSInteger code;
@property(copy) NSNumber * percentage;
- (instancetype)initWithCode:(NSInteger)code;
@end
@implementation EOBatteryLevelIntentResponse
@dynamic percentage;
+ (BOOL)supportsSecureCoding{return YES;}
- (instancetype)initWithCode:(NSInteger)code{self=[super init];if(self){_code=code;}return self;}
- (instancetype)initWithCoder:(NSCoder *)coder{self=[super initWithCoder:coder];if(self)_code=[coder decodeIntegerForKey:@"code"];return self;}
- (void)encodeWithCoder:(NSCoder *)coder{[super encodeWithCoder:coder];[coder encodeInteger:_code forKey:@"code"];}
@end
@protocol EOBatteryLevelIntentHandling<NSObject>
- (void)handleEOBatteryLevel:(EOBatteryLevelIntent *)intent completion:(void(^)(EOBatteryLevelIntentResponse *))completion;
@end
@interface EOIntentHandler:NSObject<EOBatteryLevelIntentHandling>
@end
@implementation EOIntentHandler
- (void)handleEOBatteryLevel:(EOBatteryLevelIntent *)intent completion:(void(^)(EOBatteryLevelIntentResponse *))completion{
dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{@autoreleasepool{
NSTask *task=NSTask.new;task.executableURL=[NSBundle.mainBundle executableURL];task.arguments=@[@"--action",@"status"];NSPipe *out=NSPipe.pipe;task.standardOutput=out;task.standardError=NSPipe.pipe;NSError *error=nil;BOOL launched=[task launchAndReturnError:&error];NSData *data=launched?[out.fileHandleForReading readDataToEndOfFile]:nil;if(launched)[task waitUntilExit];BOOL ok=launched&&task.terminationStatus==0;EOBatteryLevelIntentResponse *response=[[EOBatteryLevelIntentResponse alloc]initWithCode:ok?4:5];
if(ok){NSDictionary *json=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];id result=NeroGet(json,@"observation.battery.reported_charge_percent");if([result isKindOfClass:NSNumber.class])response.percentage=result;else response=[[EOBatteryLevelIntentResponse alloc]initWithCode:5];}
dispatch_async(dispatch_get_main_queue(),^{completion(response);});}});}
@end
@implementation NeroController(IntentDispatch)
- (id)application:(NSApplication *)application handlerForIntent:(INIntent *)intent{return [intent isKindOfClass:EOBatteryLevelIntent.class]?[EOIntentHandler new]:nil;}
@end

char *neroIntentAudit(long charge){@autoreleasepool{NSMutableDictionary *d=[@{@"intent_class":@(NSClassFromString(@"EOBatteryLevelIntent")!=Nil),@"response_class":@(NSClassFromString(@"EOBatteryLevelIntentResponse")!=Nil),@"handling_protocol":@(NSProtocolFromString(@"EOBatteryLevelIntentHandling")!=nil),@"handler_selector":@([EOIntentHandler instancesRespondToSelector:@selector(handleEOBatteryLevel:completion:)]),@"app_delegate_selector":@([NeroController instancesRespondToSelector:@selector(application:handlerForIntent:)]),@"model_found":@([NSBundle.mainBundle pathForResource:@"Battery" ofType:@"intentdefinition"]!=nil),@"supported": [NSBundle.mainBundle objectForInfoDictionaryKey:@"INIntentsSupported"]?:@[]}mutableCopy];
 @try{if(charge>=0&&charge<=100){EOBatteryLevelIntentResponse *r=[[EOBatteryLevelIntentResponse alloc]initWithCode:4];r.percentage=@(charge);d[@"response_property_roundtrip"]=@(r.percentage.integerValue==charge);NSError *error=nil;NSData *encoded=[NSKeyedArchiver archivedDataWithRootObject:r requiringSecureCoding:YES error:&error];d[@"response_archive"]=@(encoded!=nil&&error==nil);if(error)d[@"archive_error"]=error.description;if(encoded&&!error){EOBatteryLevelIntentResponse *decoded=[NSKeyedUnarchiver unarchivedObjectOfClass:EOBatteryLevelIntentResponse.class fromData:encoded error:&error];d[@"response_decode"]=@(decoded.code==4&&decoded.percentage.integerValue==charge);if(error)d[@"decode_error"]=error.description;}error=nil;encoded=[NSKeyedArchiver archivedDataWithRootObject:EOBatteryLevelIntent.new requiringSecureCoding:YES error:&error];d[@"intent_archive"]=@(encoded!=nil&&error==nil);if(error)d[@"intent_archive_error"]=error.description;}}@catch(NSException *e){d[@"response_exception"]=e.reason?:@"unknown";}
 NSData *data=[NSJSONSerialization dataWithJSONObject:d options:0 error:NULL];return strdup([[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding].UTF8String);}}
