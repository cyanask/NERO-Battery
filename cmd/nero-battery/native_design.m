// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"
@interface NeroInsetButtonCell:NSButtonCell
@end
@implementation NeroInsetButtonCell
- (void)drawInteriorWithFrame:(NSRect)frame inView:(NSView *)view{
 NSAttributedString *title=self.attributedTitle;NSSize text=title.size;CGFloat image=self.image?14:0,gap=image?8:0,total=ceil(text.width)+image+gap;
 CGFloat x=NSMidX(frame)-total/2,pressed=self.highlighted?1:0;
 if(self.image)[self.image drawInRect:NSMakeRect(x,NSMidY(frame)-image/2+pressed,image,image) fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:self.enabled?1:.45 respectFlipped:view.flipped hints:nil];
 [title drawInRect:NSMakeRect(x+image+gap,NSMidY(frame)-text.height/2+pressed,ceil(text.width),ceil(text.height))];
}
@end
@implementation NeroPrimaryButton
+ (Class)cellClass{return NeroInsetButtonCell.class;}
@end
@implementation NeroGauge
- (void)drawRect:(NSRect)dirty {
 NSRect r=NSInsetRect(self.bounds,10,10);NSPoint c=NSMakePoint(NSMidX(r),NSMidY(r));CGFloat radius=MIN(r.size.width,r.size.height)/2;
 NSBezierPath *track=NSBezierPath.bezierPath;[track appendBezierPathWithOvalInRect:r];track.lineWidth=7;[[[NSColor tertiaryLabelColor]colorWithAlphaComponent:.12] setStroke];[track stroke];
 NSBezierPath *arc=NSBezierPath.bezierPath;[arc appendBezierPathWithArcWithCenter:c radius:radius startAngle:90 endAngle:90-MAX(0,MIN(self.progress,100))*3.6 clockwise:YES];arc.lineWidth=7;arc.lineCapStyle=NSLineCapStyleRound;[NeroAccent()setStroke];[arc stroke];
 NSImage *mark=[NSImage imageNamed:@"MenuMarkTemplate"];NSImage *tinted=[NSImage imageWithSize:NSMakeSize(28,28) flipped:NO drawingHandler:^BOOL(NSRect rect){[mark drawInRect:rect];[NSColor.labelColor setFill];NSRectFillUsingOperation(rect,NSCompositingOperationSourceIn);return YES;}];[tinted drawInRect:NSMakeRect(c.x-14,c.y-14,28,28)];
}
@end
NSColor *NeroAccent(void){return [NSColor colorWithSRGBRed:.09 green:.53 blue:.39 alpha:1];}
void NeroPrimaryTitle(NSButton *button,NSString *title){
 if(!button)return;button.bordered=NO;button.wantsLayer=YES;button.layer.cornerRadius=7;button.layer.backgroundColor=[NSColor colorWithSRGBRed:.07 green:.40 blue:.29 alpha:1].CGColor;button.contentTintColor=NSColor.whiteColor;
 if([button.attributedTitle.string isEqualToString:title]&&button.attributedTitle.length&&[[button.attributedTitle attribute:NSForegroundColorAttributeName atIndex:0 effectiveRange:NULL]isEqual:NSColor.whiteColor])return;
 NSMutableParagraphStyle *paragraph=NSMutableParagraphStyle.new;paragraph.alignment=NSTextAlignmentCenter;button.attributedTitle=[[NSAttributedString alloc]initWithString:title attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:12 weight:NSFontWeightSemibold],NSForegroundColorAttributeName:NSColor.whiteColor,NSParagraphStyleAttributeName:paragraph}];
}
NSStackView *NeroCard(NSStackView *parent){
 NSBox *box=NSBox.new;box.boxType=NSBoxCustom;box.titlePosition=NSNoTitle;box.borderWidth=.5;box.borderColor=[NSColor.separatorColor colorWithAlphaComponent:.35];box.fillColor=NSColor.controlBackgroundColor;box.cornerRadius=10;box.contentViewMargins=NSZeroSize;
 NSStackView *s=NSStackView.new;s.orientation=NSUserInterfaceLayoutOrientationVertical;s.alignment=NSLayoutAttributeLeading;s.spacing=8;s.translatesAutoresizingMaskIntoConstraints=NO;[box.contentView addSubview:s];
 [NSLayoutConstraint activateConstraints:@[[s.leadingAnchor constraintEqualToAnchor:box.contentView.leadingAnchor constant:14],[s.trailingAnchor constraintEqualToAnchor:box.contentView.trailingAnchor constant:-14],[s.topAnchor constraintEqualToAnchor:box.contentView.topAnchor constant:12],[s.bottomAnchor constraintEqualToAnchor:box.contentView.bottomAnchor constant:-12]]];[parent addArrangedSubview:box];NeroFullWidth(box,parent);return s;
}
