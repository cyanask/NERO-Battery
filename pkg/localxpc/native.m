// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import <Foundation/Foundation.h>
#import "native.h"
#include <stdlib.h>
#include <string.h>

// Original typed NSXPC protocol; only bounded command names and JSON data cross it.
@protocol NeroBatteryControl
- (void)command:(NSString *)command payload:(NSData *)payload reply:(void (^)(NSInteger,NSData *))reply;
@end
extern void neroXPCDispatch(char *,void *,size_t,int *,void **,size_t *);
static NSString *const serviceName=@"io.nero.battery.helper";
static const NSUInteger payloadLimit=16*1024,replyLimit=2*1024*1024;

@interface NeroConnectionHandler:NSObject<NeroBatteryControl>
@property(weak) NSXPCConnection *peer;
@property uid_t authorizedUID;
@end
@implementation NeroConnectionHandler
- (void)command:(NSString *)command payload:(NSData *)payload reply:(void (^)(NSInteger,NSData *))reply {
 NSXPCConnection *peer=self.peer;
 if(!peer||peer.effectiveUserIdentifier!=self.authorizedUID){reply(403,[NSData data]);return;}
 if(command.length>64||payload.length>payloadLimit){reply(413,[NSData data]);return;}
 int status=500;void *bytes=NULL;size_t size=0;
 neroXPCDispatch((char *)command.UTF8String,(void *)payload.bytes,payload.length,&status,&bytes,&size);
 NSData *result=size<=replyLimit?[NSData dataWithBytes:bytes length:size]:[NSData data];
 free(bytes);reply(size<=replyLimit?status:413,result);
}
@end

@interface NeroXPCServer:NSObject<NSXPCListenerDelegate>
@property NSXPCListener *listener;
@property NSString *requirement;
@property uid_t authorizedUID;
@property NSMutableSet<NSXPCConnection *> *connections;
@end
@implementation NeroXPCServer
- (BOOL)listener:(NSXPCListener *)listener shouldAcceptNewConnection:(NSXPCConnection *)connection {
 if(connection.effectiveUserIdentifier!=self.authorizedUID)return NO;
 // Public Foundation API checks the kernel peer identity, not a caller PID.
 [connection setCodeSigningRequirement:self.requirement];
 connection.exportedInterface=[NSXPCInterface interfaceWithProtocol:@protocol(NeroBatteryControl)];
 NeroConnectionHandler *handler=NeroConnectionHandler.new;handler.peer=connection;handler.authorizedUID=self.authorizedUID;connection.exportedObject=handler;
 __weak NeroXPCServer *weakSelf=self;__weak NSXPCConnection *weakConnection=connection;
 connection.invalidationHandler=^{NeroXPCServer *s=weakSelf;NSXPCConnection *c=weakConnection;if(s&&c)@synchronized(s.connections){[s.connections removeObject:c];}};
 @synchronized(self.connections){[self.connections addObject:connection];}
 [connection resume];return YES;
}
@end
static NeroXPCServer *server;
int neroXPCStart(unsigned uid,const char *requirement,char **error){@autoreleasepool{
 @try {
  if(server){*error=strdup("XPC listener already active");return 0;}
  NeroXPCServer *s=NeroXPCServer.new;s.authorizedUID=uid;s.requirement=@(requirement);s.connections=NSMutableSet.new;
  s.listener=[[NSXPCListener alloc]initWithMachServiceName:serviceName];
  [s.listener setConnectionCodeSigningRequirement:s.requirement];s.listener.delegate=s;server=s;[s.listener resume];return 1;
 } @catch(NSException *e){*error=strdup("XPC listener initialization failed");server=nil;return 0;}
}}
void neroXPCStop(void){@autoreleasepool{
 NeroXPCServer *s=server;if(!s)return;[s.listener invalidate];NSArray *connections;@synchronized(s.connections){connections=s.connections.allObjects;[s.connections removeAllObjects];}
 for(NSXPCConnection *connection in connections)[connection invalidate];server=nil;
}}

void *neroXPCRequest(const char *requirement,const char *name,const void *payload,size_t payloadLength,int *status,size_t *length,char **error){@autoreleasepool{
 *status=503;*length=0;*error=NULL;if(payloadLength>payloadLimit){*error=strdup("XPC payload exceeds limit");return NULL;}
 @try {
  NSXPCConnection *connection=[[NSXPCConnection alloc]initWithMachServiceName:serviceName options:NSXPCConnectionPrivileged];
  connection.remoteObjectInterface=[NSXPCInterface interfaceWithProtocol:@protocol(NeroBatteryControl)];[connection setCodeSigningRequirement:@(requirement)];[connection resume];
  dispatch_semaphore_t semaphore=dispatch_semaphore_create(0);NSLock *lock=NSLock.new;
  __block BOOL finished=NO;__block NSData *result=nil;__block NSInteger code=503;__block NSString *failure=nil;
  void (^complete)(NSInteger,NSData *,NSString *)=^(NSInteger c,NSData *data,NSString *message){[lock lock];if(!finished){finished=YES;code=c;result=data;failure=message;dispatch_semaphore_signal(semaphore);}[lock unlock];};
  id<NeroBatteryControl> proxy=[connection remoteObjectProxyWithErrorHandler:^(NSError *e){complete(503,nil,[NSString stringWithFormat:@"XPC connection rejected or unavailable (%@/%ld)",e.domain,(long)e.code]);}];
  [proxy command:@(name) payload:[NSData dataWithBytes:payload length:payloadLength] reply:^(NSInteger c,NSData *data){if(connection.effectiveUserIdentifier!=0){complete(403,nil,@"XPC server account rejected");return;}if(data.length>replyLimit){complete(413,nil,@"XPC reply exceeds limit");return;}complete(c,data,nil);}];
  if(dispatch_semaphore_wait(semaphore,dispatch_time(DISPATCH_TIME_NOW,12*NSEC_PER_SEC))!=0)complete(504,nil,@"XPC request timed out");
  [connection invalidate];[lock lock];*status=(int)code;if(failure)*error=strdup(failure.UTF8String);void *bytes=NULL;if(result.length){*length=result.length;bytes=malloc(result.length);if(bytes)memcpy(bytes,result.bytes,result.length);else{*length=0;*error=strdup("XPC reply allocation failed");}}[lock unlock];return bytes;
 } @catch(NSException *e){*error=strdup("XPC request failed");return NULL;}
}}
