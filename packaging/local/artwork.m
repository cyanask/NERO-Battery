// NDT original vector artwork: Energy Orbit. No letters or third-party assets.
#import <Cocoa/Cocoa.h>
static NSColor *rgb(CGFloat r,CGFloat g,CGFloat b){return [NSColor colorWithSRGBRed:r green:g blue:b alpha:1];}
static NSBezierPath *bolt(NSRect r){
 NSBezierPath *p=NSBezierPath.bezierPath;
 #define P(px,py) NSMakePoint(r.origin.x+r.size.width*(px),r.origin.y+r.size.height*(py))
 [p moveToPoint:P(.57,.84)];[p curveToPoint:P(.61,.81) controlPoint1:P(.60,.87) controlPoint2:P(.63,.85)];
 [p lineToPoint:P(.54,.58)];[p lineToPoint:P(.70,.58)];[p curveToPoint:P(.72,.53) controlPoint1:P(.73,.58) controlPoint2:P(.74,.55)];
 [p lineToPoint:P(.43,.18)];[p curveToPoint:P(.39,.21) controlPoint1:P(.40,.15) controlPoint2:P(.37,.17)];
 [p lineToPoint:P(.46,.44)];[p lineToPoint:P(.30,.44)];[p curveToPoint:P(.28,.49) controlPoint1:P(.27,.44) controlPoint2:P(.26,.47)];[p closePath];
 #undef P
 return p;
}
static NSBezierPath *orbit(NSRect r){NSBezierPath *p=NSBezierPath.bezierPath;CGFloat d=MIN(r.size.width,r.size.height);[p appendBezierPathWithArcWithCenter:NSMakePoint(NSMidX(r),NSMidY(r)) radius:d*.386 startAngle:49 endAngle:311 clockwise:NO];p.lineWidth=d*.066;p.lineCapStyle=NSLineCapStyleRound;return p;}
static void mark(NSRect r,NSColor *ink){[ink setFill];[bolt(r)fill];[ink setStroke];[orbit(r)stroke];}
static void appIcon(NSInteger size){
 CGFloat s=size;NSRect tile=NSMakeRect(s*.078,s*.078,s*.844,s*.844);
 NSBezierPath *base=[NSBezierPath bezierPathWithRoundedRect:tile xRadius:s*.19 yRadius:s*.19];
 [NSGraphicsContext saveGraphicsState];NSShadow *shadow=NSShadow.new;shadow.shadowColor=[NSColor colorWithWhite:0 alpha:.15];shadow.shadowBlurRadius=s*.025;shadow.shadowOffset=NSMakeSize(0,-s*.012);[shadow set];[rgb(.90,.94,.92)setFill];[base fill];[NSGraphicsContext restoreGraphicsState];
 [[[NSGradient alloc]initWithStartingColor:rgb(1,1,.99) endingColor:rgb(.87,.93,.90)]drawInBezierPath:base angle:285];
 NSBezierPath *edge=[NSBezierPath bezierPathWithRoundedRect:NSInsetRect(tile,s*.005,s*.005) xRadius:s*.184 yRadius:s*.184];edge.lineWidth=MAX(.5,s*.003);[[NSColor colorWithWhite:1 alpha:.8]setStroke];[edge stroke];
 NSBezierPath *well=[NSBezierPath bezierPathWithOvalInRect:NSMakeRect(s*.226,s*.226,s*.548,s*.548)];
 [[[NSGradient alloc]initWithStartingColor:rgb(.86,.96,.91) endingColor:rgb(.96,.99,.96)]drawInBezierPath:well angle:80];
 NSRect symbol=NSMakeRect(s*.155,s*.155,s*.69,s*.69);
 [NSGraphicsContext saveGraphicsState];NSShadow *lift=NSShadow.new;lift.shadowColor=[NSColor colorWithSRGBRed:0 green:.23 blue:.15 alpha:.23];lift.shadowBlurRadius=s*.014;lift.shadowOffset=NSMakeSize(0,-s*.012);[lift set];mark(symbol,rgb(.05,.40,.29));[NSGraphicsContext restoreGraphicsState];
 [[[NSGradient alloc]initWithStartingColor:rgb(.13,.72,.49) endingColor:rgb(.015,.36,.27)]drawInBezierPath:bolt(symbol) angle:285];
 [NSGraphicsContext saveGraphicsState];[bolt(symbol)addClip];NSBezierPath *face=NSBezierPath.bezierPath;
 [face moveToPoint:NSMakePoint(s*.61,s*.80)];[face lineToPoint:NSMakePoint(s*.50,s*.50)];[face lineToPoint:NSMakePoint(s*.41,s*.27)];[face lineToPoint:NSMakePoint(s*.22,s*.78)];[face closePath];[[NSColor colorWithWhite:1 alpha:.16]setFill];[face fill];[NSGraphicsContext restoreGraphicsState];
}
static void png(NSString *path,NSInteger w,NSInteger h,void(^draw)(void)){
 NSBitmapImageRep *rep=[[NSBitmapImageRep alloc]initWithBitmapDataPlanes:NULL pixelsWide:w pixelsHigh:h bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO colorSpaceName:NSDeviceRGBColorSpace bytesPerRow:0 bitsPerPixel:0];[NSGraphicsContext saveGraphicsState];NSGraphicsContext.currentContext=[NSGraphicsContext graphicsContextWithBitmapImageRep:rep];[NSColor.clearColor setFill];NSRectFill(NSMakeRect(0,0,w,h));draw();[NSGraphicsContext restoreGraphicsState];if(![[rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}]writeToFile:path atomically:YES])exit(1);
}
static void label(NSString *s,CGFloat x,CGFloat y,CGFloat size){[s drawAtPoint:NSMakePoint(x,y)withAttributes:@{NSFontAttributeName:[NSFont systemFontOfSize:size weight:NSFontWeightMedium],NSForegroundColorAttributeName:rgb(.20,.28,.24)}];}
int main(int argc,const char *argv[]){@autoreleasepool{
 if(argc!=3)return 2;NSString *folder=@(argv[1]),*preview=@(argv[2]),*set=[folder stringByAppendingPathComponent:@"Nero.iconset"];
 [NSFileManager.defaultManager createDirectoryAtPath:set withIntermediateDirectories:YES attributes:nil error:NULL];
 for(NSNumber *n in @[@16,@32,@128,@256,@512]){NSInteger s=n.integerValue;png([set stringByAppendingPathComponent:[NSString stringWithFormat:@"icon_%ldx%ld.png",s,s]],s,s,^{appIcon(s);});png([set stringByAppendingPathComponent:[NSString stringWithFormat:@"icon_%ldx%ld@2x.png",s,s]],s*2,s*2,^{appIcon(s*2);});}
 for(NSNumber *n in @[@1,@2]){NSInteger scale=n.integerValue;png([folder stringByAppendingPathComponent:scale==1?@"MenuMarkTemplate.png":@"MenuMarkTemplate@2x.png"],18*scale,18*scale,^{mark(NSMakeRect(0,0,18*scale,18*scale),NSColor.blackColor);});}
 png(preview,920,400,^{[rgb(.955,.965,.96)setFill];NSRectFill(NSMakeRect(0,0,920,400));label(@"能量环 · Energy Orbit",36,345,24);
 [NSGraphicsContext saveGraphicsState];NSAffineTransform *t=NSAffineTransform.transform;[t translateXBy:30 yBy:65];[t concat];appIcon(260);[NSGraphicsContext restoreGraphicsState];label(@"App",136,38,13);
 label(@"状态栏 · 实际 18 pt",355,280,17);[rgb(.86,.88,.87)setFill];NSRectFill(NSMakeRect(355,207,220,48));mark(NSMakeRect(456,222,18,18),rgb(.12,.14,.13));[rgb(.12,.14,.13)setFill];NSRectFill(NSMakeRect(355,145,220,48));mark(NSMakeRect(456,160,18,18),NSColor.whiteColor);label(@"浅色 / 深色自动适配",355,112,13);
 label(@"小尺寸",645,280,17);for(NSNumber *n in @[@64,@32,@16]){NSInteger s=n.integerValue;CGFloat x=s==64?635:s==32?735:807;[NSGraphicsContext saveGraphicsState];NSAffineTransform *t=NSAffineTransform.transform;[t translateXBy:x yBy:190];[t concat];appIcon(s);[NSGraphicsContext restoreGraphicsState];label([NSString stringWithFormat:@"%ld",s],x+s*.25,165,12);}label(@"无字母标识 · 原创矢量",645,112,13);
 });}return 0;}
