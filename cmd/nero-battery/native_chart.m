// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"

@implementation NeroChart
- (BOOL)isFlipped{return YES;}
- (BOOL)isAccessibilityElement{return YES;}
- (NSString *)accessibilityRole{return NSAccessibilityImageRole;}
- (void)drawRect:(NSRect)dirty{
 NSRect b=self.bounds;[NSColor.controlBackgroundColor setFill];[[NSBezierPath bezierPathWithRoundedRect:NSInsetRect(b,1,1) xRadius:10 yRadius:10]fill];
 NSArray *points=self.points?:@[];NSString *key=self.metric?:@"charge";
 NSString *unit=[key isEqualToString:@"charge"]?@"%":([key isEqualToString:@"temperature_c"]?@"°C":@"W");
 NSMutableArray *valid=[NSMutableArray array];double maximum=0;
 for(NSDictionary *p in points){NSNumber *v=NeroNumber(p[key]);NSDate *date=(p[@"_date"]?:NeroDate(p[@"at"]));if(v&&date){maximum=MAX(maximum,v.doubleValue);[valid addObject:p];}}
 if(!valid.count){NSString *text=points.count?@"此项暂无有效读数":@"正在积累本机历史数据…";[text drawAtPoint:NSMakePoint(40,b.size.height*.45) withAttributes:@{NSFontAttributeName:[NSFont systemFontOfSize:15],NSForegroundColorAttributeName:NSColor.secondaryLabelColor}];return;}
 NSRect plot=NSMakeRect(52,25,MAX(10,b.size.width-76),MAX(10,b.size.height-72));
 double top=[key isEqualToString:@"charge"]?100:([key isEqualToString:@"temperature_c"]?MAX(60,ceil(maximum/20)*20):MAX(20,ceil(maximum/20)*20));
 NSDictionary *attrs=@{NSFontAttributeName:[NSFont monospacedDigitSystemFontOfSize:11 weight:NSFontWeightRegular],NSForegroundColorAttributeName:NSColor.secondaryLabelColor};
 for(int i=0;i<=4;i++){CGFloat y=plot.origin.y+plot.size.height*i/4;NSBezierPath *line=[NSBezierPath bezierPath];[line moveToPoint:NSMakePoint(plot.origin.x,y)];[line lineToPoint:NSMakePoint(NSMaxX(plot),y)];line.lineWidth=.5;[NSColor.separatorColor setStroke];[line stroke];NSString *label=[NSString stringWithFormat:@"%.0f%@",top*(4-i)/4,unit];[label drawAtPoint:NSMakePoint(6,y-6) withAttributes:attrs];}
 NSDate *first=NeroDate(points.firstObject[@"at"]),*last=NeroDate(points.lastObject[@"at"]);if(!first||!last)return;NSTimeInterval start=first.timeIntervalSince1970,end=MAX(last.timeIntervalSince1970,start+60);
 NSDateFormatter *dateFmt=[[NSDateFormatter alloc]init];dateFmt.dateFormat=end-start>=48*3600?@"MM-dd":(end-start>=12*3600?@"MM-dd HH:mm":@"HH:mm");
 for(int i=0;i<4;i++){NSTimeInterval t=start+(end-start)*i/3;NSString *label=[dateFmt stringFromDate:[NSDate dateWithTimeIntervalSince1970:t]];[label drawAtPoint:NSMakePoint(plot.origin.x+plot.size.width*i/3-15,NSMaxY(plot)+13) withAttributes:attrs];}
 NSBezierPath *path=[NSBezierPath bezierPath];path.lineWidth=2.3;BOOL continuing=NO;
 for(NSDictionary *p in points){NSNumber *value=NeroNumber(p[key]);NSDate *date=(p[@"_date"]?:NeroDate(p[@"at"]));if(!value||!date){continuing=NO;continue;}CGFloat x=plot.origin.x+plot.size.width*(date.timeIntervalSince1970-start)/(end-start),y=NSMaxY(plot)-plot.size.height*value.doubleValue/top;NSPoint point=NSMakePoint(x,y);if([p[@"break_before"]boolValue])continuing=NO;if(continuing)[path lineToPoint:point];else[path moveToPoint:point];continuing=YES;}
 [NSColor.controlAccentColor setStroke];[path stroke];
 if(valid.count==1){NSDictionary *p=valid.firstObject;CGFloat x=plot.origin.x,y=NSMaxY(plot)-plot.size.height*[p[key]doubleValue]/top;[NSColor.controlAccentColor setFill];[[NSBezierPath bezierPathWithOvalInRect:NSMakeRect(x-3,y-3,6,6)]fill];}
 self.accessibilityLabel=[NSString stringWithFormat:@"本机历史图表，%lu 个有效读数，最新值 %@%@",(unsigned long)valid.count,valid.lastObject[key],unit];
}
@end
