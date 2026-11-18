// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"

NeroController *neroController;
@interface NeroDocumentView:NSView
@end
@implementation NeroDocumentView
- (BOOL)isFlipped{return YES;}
@end

NSTextField *NeroLabel(NSString *text,CGFloat size,NSFontWeight weight){
 NSTextField *v=[NSTextField wrappingLabelWithString:text?:@""];v.font=[NSFont systemFontOfSize:size weight:weight];v.selectable=YES;return v;
}
NSStackView *NeroRow(NSArray<NSView *> *views){NSStackView *s=[NSStackView stackViewWithViews:views];s.orientation=NSUserInterfaceLayoutOrientationHorizontal;s.spacing=8;s.alignment=NSLayoutAttributeCenterY;return s;}
void NeroFullWidth(NSView *v,NSStackView *s){[v.widthAnchor constraintEqualToAnchor:s.widthAnchor].active=YES;}
void NeroSection(NSStackView *s,NSString *title,NSString *note){
 if(s.arrangedSubviews.count)[s setCustomSpacing:8 afterView:s.arrangedSubviews.lastObject];
 [s addArrangedSubview:NeroLabel(title,14,NSFontWeightSemibold)];
 if(note.length){NSTextField *n=NeroLabel(note,12,NSFontWeightRegular);n.textColor=NSColor.secondaryLabelColor;[s addArrangedSubview:n];NeroFullWidth(n,s);}
}
NSButton *NeroButton(NeroController *o,NSString *title,NSInteger action){
 NSButton *b=[NSButton buttonWithTitle:title target:o action:@selector(act:)];b.tag=action;b.bezelStyle=NSBezelStyleRounded;o.controls[@(action)]=b;return b;
}
NSPopUpButton *NeroPopup(NSArray<NSString *> *items,NSInteger selected){NSPopUpButton *p=[[NSPopUpButton alloc]initWithFrame:NSZeroRect pullsDown:NO];[p addItemsWithTitles:items];if(selected>=0&&selected<p.numberOfItems)[p selectItemAtIndex:selected];return p;}
void NeroValue(NeroController *o,NSString *key,NSString *value){NSTextField *v=o.values[key];NSString *text=value?:@"—";if(v&&![v.stringValue isEqualToString:text])v.stringValue=text;}
id NeroGet(NSDictionary *json,NSString *path){id v=json;for(NSString *part in[path componentsSeparatedByString:@"."]){if(![v isKindOfClass:NSDictionary.class])return nil;v=v[part];if(v==NSNull.null)return nil;}return v;}
NSNumber *NeroNumber(id value){return[value isKindOfClass:NSNumber.class]?value:nil;}
NSString *NeroMetricString(id value,NSString *unit,NSInteger decimals){NSNumber *n=NeroNumber(value);if(!n)return @"—";return[NSString stringWithFormat:@"%.*f%@",(int)decimals,n.doubleValue,unit];}
NSDate *NeroDate(NSString *text){if(![text isKindOfClass:NSString.class]||[text hasPrefix:@"0001"]||!text.length)return nil;static NSISO8601DateFormatter *fractional,*plain;static dispatch_once_t once;dispatch_once(&once,^{fractional=[[NSISO8601DateFormatter alloc]init];fractional.formatOptions=NSISO8601DateFormatWithInternetDateTime|NSISO8601DateFormatWithFractionalSeconds;plain=[[NSISO8601DateFormatter alloc]init];plain.formatOptions=NSISO8601DateFormatWithInternetDateTime;});return[fractional dateFromString:text]?:[plain dateFromString:text];}
void NeroSend(NSInteger action,id value){
 NSString *payload=@"";if([value isKindOfClass:NSString.class]&&action==30)payload=value;else if(value){NSData *data=[NSJSONSerialization dataWithJSONObject:value options:NSJSONWritingFragmentsAllowed error:NULL];if(!data)return;payload=[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding];}
 neroPerform((int)action,payload.UTF8String);
}

@implementation NeroController
- (NSStackView *)newPanel{
 NSScrollView *scroll=[[NSScrollView alloc]init];scroll.hasVerticalScroller=YES;scroll.drawsBackground=NO;scroll.translatesAutoresizingMaskIntoConstraints=NO;
 NSView *doc=[[NeroDocumentView alloc]init];doc.translatesAutoresizingMaskIntoConstraints=NO;scroll.documentView=doc;
 NSStackView *stack=[[NSStackView alloc]init];stack.orientation=NSUserInterfaceLayoutOrientationVertical;stack.alignment=NSLayoutAttributeLeading;stack.spacing=8;stack.translatesAutoresizingMaskIntoConstraints=NO;[doc addSubview:stack];
 [NSLayoutConstraint activateConstraints:@[[doc.widthAnchor constraintEqualToAnchor:scroll.contentView.widthAnchor],[stack.leadingAnchor constraintEqualToAnchor:doc.leadingAnchor constant:20],[stack.trailingAnchor constraintEqualToAnchor:doc.trailingAnchor constant:-20],[stack.topAnchor constraintEqualToAnchor:doc.topAnchor constant:12],[stack.bottomAnchor constraintEqualToAnchor:doc.bottomAnchor constant:-16]]];
 [self.host addSubview:scroll];[NSLayoutConstraint activateConstraints:@[[scroll.leadingAnchor constraintEqualToAnchor:self.host.leadingAnchor],[scroll.trailingAnchor constraintEqualToAnchor:self.host.trailingAnchor],[scroll.topAnchor constraintEqualToAnchor:self.host.topAnchor],[scroll.bottomAnchor constraintEqualToAnchor:self.host.bottomAnchor]]];scroll.hidden=YES;[self.panels addObject:scroll];return stack;
}
- (void)applicationDidFinishLaunching:(NSNotification *)note{
 NSImage *appArtwork=[[NSImage alloc]initWithContentsOfFile:[NSBundle.mainBundle pathForResource:@"AppIcon" ofType:@"icns"]];if(appArtwork)NSApp.applicationIconImage=appArtwork;
 self.values=[NSMutableDictionary dictionary];self.controls=[NSMutableDictionary dictionary];self.panels=[NSMutableArray array];self.navigation=[NSMutableArray array];self.rules=@[];self.lastStatus=@{};
 self.window=[[NSWindow alloc]initWithContentRect:NSMakeRect(0,0,1080,780) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
 self.window.title=@"NERO Battery Management";self.window.titleVisibility=NSWindowTitleHidden;self.window.titlebarAppearsTransparent=YES;self.window.backgroundColor=NSColor.underPageBackgroundColor;self.window.minSize=NSMakeSize(940,660);self.window.releasedWhenClosed=NO;self.window.delegate=self;
 NSTitlebarAccessoryViewController *identity=NSTitlebarAccessoryViewController.new;identity.layoutAttribute=NSLayoutAttributeLeft;identity.view=[[NSView alloc]initWithFrame:NSMakeRect(0,0,254,28)];
 NSMutableAttributedString *title=[[NSMutableAttributedString alloc]initWithString:@"NERO Battery Management" attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:12.5 weight:NSFontWeightMedium],NSForegroundColorAttributeName:NSColor.labelColor,NSKernAttributeName:@.15}];[title addAttributes:@{NSFontAttributeName:[NSFont systemFontOfSize:12.5 weight:NSFontWeightSemibold],NSKernAttributeName:@.65} range:NSMakeRange(0,4)];
 NSTextField *titleLabel=[NSTextField labelWithAttributedString:title];titleLabel.frame=NSMakeRect(8,4,240,20);[identity.view addSubview:titleLabel];[self.window addTitlebarAccessoryViewController:identity];
 NSView *root=self.window.contentView;
 NSVisualEffectView *sidebar=[[NSVisualEffectView alloc]init];sidebar.material=NSVisualEffectMaterialSidebar;sidebar.blendingMode=NSVisualEffectBlendingModeBehindWindow;sidebar.translatesAutoresizingMaskIntoConstraints=NO;[root addSubview:sidebar];
 NSStackView *nav=[[NSStackView alloc]init];nav.orientation=NSUserInterfaceLayoutOrientationVertical;nav.alignment=NSLayoutAttributeLeading;nav.spacing=4;nav.translatesAutoresizingMaskIntoConstraints=NO;[sidebar addSubview:nav];
 NSArray *names=@[@"总览",@"充电与保护",@"电池校准",@"历史趋势",@"定时任务",@"能耗与电源",@"睡眠与合盖"];
 NSArray *symbols=@[@"square.grid.2x2",@"bolt.shield",@"arrow.triangle.2.circlepath",@"chart.xyaxis.line",@"calendar",@"waveform.path.ecg",@"moon.zzz"];
 for(NSNumber *index in @[@0,@1,@2,@6,@3,@4,@5]){NSInteger i=index.integerValue;
 if(i==1||i==3){NSTextField *group=NeroLabel(i==1?@"电池保养":@"记录与自动化",11,NSFontWeightMedium);group.textColor=NSColor.secondaryLabelColor;[nav addArrangedSubview:group];}
 NSButton *b=[NSButton buttonWithTitle:names[i] target:self action:@selector(navigate:)];b.tag=i;b.image=[NSImage imageWithSystemSymbolName:symbols[i] accessibilityDescription:nil];b.imagePosition=NSImageLeading;b.alignment=NSTextAlignmentLeft;b.bezelStyle=NSBezelStyleRounded;b.bordered=NO;b.wantsLayer=YES;b.layer.cornerRadius=8;b.font=[NSFont systemFontOfSize:13 weight:NSFontWeightMedium];[b.widthAnchor constraintEqualToConstant:146].active=YES;[b.heightAnchor constraintEqualToConstant:32].active=YES;[nav addArrangedSubview:b];[self.navigation addObject:b];}
 NSTextField *private=NeroLabel(@"数据仅存本机\n无共享、上传或在线更新",11,NSFontWeightRegular);private.textColor=NSColor.secondaryLabelColor;private.translatesAutoresizingMaskIntoConstraints=NO;[sidebar addSubview:private];[NSLayoutConstraint activateConstraints:@[[private.leadingAnchor constraintEqualToAnchor:sidebar.leadingAnchor constant:14],[private.trailingAnchor constraintEqualToAnchor:sidebar.trailingAnchor constant:-14],[private.bottomAnchor constraintEqualToAnchor:sidebar.bottomAnchor constant:-18]]];
 NSView *right=[[NSView alloc]init];right.translatesAutoresizingMaskIntoConstraints=NO;[root addSubview:right];
 self.connection=NeroLabel(@"正在连接后台…",11,NSFontWeightMedium);self.connection.textColor=NSColor.secondaryLabelColor;
 self.feedback=NeroLabel(@"",12,NSFontWeightRegular);self.feedback.textColor=NSColor.secondaryLabelColor;self.feedback.translatesAutoresizingMaskIntoConstraints=NO;[right addSubview:self.feedback];
 self.host=[[NSView alloc]init];self.host.translatesAutoresizingMaskIntoConstraints=NO;[right addSubview:self.host];
 [NSLayoutConstraint activateConstraints:@[[sidebar.leadingAnchor constraintEqualToAnchor:root.leadingAnchor],[sidebar.topAnchor constraintEqualToAnchor:root.topAnchor],[sidebar.bottomAnchor constraintEqualToAnchor:root.bottomAnchor],[sidebar.widthAnchor constraintEqualToConstant:174],[nav.leadingAnchor constraintEqualToAnchor:sidebar.leadingAnchor constant:14],[nav.topAnchor constraintEqualToAnchor:sidebar.topAnchor constant:16],[nav.trailingAnchor constraintEqualToAnchor:sidebar.trailingAnchor constant:-14],[right.leadingAnchor constraintEqualToAnchor:sidebar.trailingAnchor],[right.trailingAnchor constraintEqualToAnchor:root.trailingAnchor],[right.topAnchor constraintEqualToAnchor:root.topAnchor],[right.bottomAnchor constraintEqualToAnchor:root.bottomAnchor],[self.host.leadingAnchor constraintEqualToAnchor:right.leadingAnchor],[self.host.trailingAnchor constraintEqualToAnchor:right.trailingAnchor],[self.host.topAnchor constraintEqualToAnchor:right.topAnchor],[self.host.bottomAnchor constraintEqualToAnchor:self.feedback.topAnchor constant:-4],[self.feedback.leadingAnchor constraintEqualToAnchor:right.leadingAnchor constant:20],[self.feedback.trailingAnchor constraintEqualToAnchor:right.trailingAnchor constant:-20],[self.feedback.bottomAnchor constraintEqualToAnchor:right.bottomAnchor constant:-10],[self.feedback.heightAnchor constraintGreaterThanOrEqualToConstant:20]]];
 [self buildDashboard];[self buildCharging];[self buildCalibration];[self buildHistory];[self buildAutomation];[self buildEnergy];[self buildSleep];
 [self navigate:self.navigation[0]];
 NSMenu *bar=[[NSMenu alloc]init];NSMenuItem *appItem=[[NSMenuItem alloc]init];[bar addItem:appItem];NSMenu *menu=[[NSMenu alloc]init];
 NSMenuItem *show=[[NSMenuItem alloc]initWithTitle:@"打开应用" action:@selector(show:) keyEquivalent:@"0"];show.target=self;[menu addItem:show];[menu addItem:NSMenuItem.separatorItem];[menu addItemWithTitle:@"退出界面（保留后台控制）" action:@selector(terminate:) keyEquivalent:@"q"];appItem.submenu=menu;NSApp.mainMenu=bar;
 self.statusItem=[NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength];NSImage *mark=[NSImage imageNamed:@"MenuMarkTemplate"];mark.template=YES;mark.size=NSMakeSize(18,18);self.statusItem.button.image=mark;self.statusItem.button.accessibilityLabel=@"NERO 电池控制";
 NSMenu *statusMenu=[[NSMenu alloc]init];statusMenu.autoenablesItems=NO;self.menuStatus=[[NSMenuItem alloc]initWithTitle:@"正在读取状态…" action:nil keyEquivalent:@""];[statusMenu addItem:self.menuStatus];NSMenuItem *statusShow=[[NSMenuItem alloc]initWithTitle:@"打开应用" action:@selector(show:) keyEquivalent:@""];statusShow.target=self;[statusMenu addItem:statusShow];[statusMenu addItem:NSMenuItem.separatorItem];
 self.menuFull=[[NSMenuItem alloc]initWithTitle:@"临时充到 100%" action:@selector(act:) keyEquivalent:@""];self.menuFull.target=self;self.menuFull.tag=2;[statusMenu addItem:self.menuFull];self.menuStop=[[NSMenuItem alloc]initWithTitle:@"停止手动放电" action:@selector(act:) keyEquivalent:@""];self.menuStop.target=self;self.menuStop.tag=5;[statusMenu addItem:self.menuStop];statusMenu.delegate=self;self.statusItem.menu=statusMenu;
 [self.window center];[self show:nil];[self updateUIRefreshTimer];[self refresh:nil];
}
- (void)show:(id)sender{[self.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];self.connection.stringValue=@"正在刷新本机状态…";[self updateUIRefreshTimer];[self refresh:nil];}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)app hasVisibleWindows:(BOOL)visible{[self show:nil];return YES;}
- (void)navigate:(NSButton *)sender{self.selectedPanel=sender.tag;for(NSInteger i=0;i<self.panels.count;i++)self.panels[i].hidden=i!=sender.tag;for(NSButton *button in self.navigation){BOOL selected=button.tag==sender.tag;button.state=selected?NSControlStateValueOn:NSControlStateValueOff;button.layer.backgroundColor=(selected?[NeroAccent() colorWithAlphaComponent:.15]:NSColor.clearColor).CGColor;button.contentTintColor=NSColor.labelColor;}if(sender.tag==3)[self historyChanged:nil];if(sender.tag==5&&self.connected)NeroSend(23,nil);}
// The controller keeps charging independently. UI polling only runs while used.
- (BOOL)uiNeedsRefresh{return (self.window.visible&&!self.window.miniaturized&&!NSApp.hidden)||self.menuTracking;}
- (void)updateUIRefreshTimer{BOOL active=[self uiNeedsRefresh];neroControlClientActive(active);if(active){if(!self.timer){self.timer=[NSTimer scheduledTimerWithTimeInterval:5 target:self selector:@selector(refresh:) userInfo:nil repeats:YES];self.timer.tolerance=1;}}else{[self.timer invalidate];self.timer=nil;}}
- (void)refresh:(id)sender{if(![self uiNeedsRefresh])return;neroRefresh();if(self.window.visible&&!self.window.miniaturized&&!NSApp.hidden&&self.selectedPanel==3&&(!self.historyRefreshAt||-[self.historyRefreshAt timeIntervalSinceNow]>30))[self historyChanged:nil];}
- (void)windowWillClose:(NSNotification *)note{[self.timer invalidate];self.timer=nil;neroControlClientActive(self.menuTracking);self.statusItem.button.toolTip=@"电池控制 · 点击查看最新状态";}
- (void)applicationDidHide:(NSNotification *)note{[self updateUIRefreshTimer];}
- (void)applicationDidUnhide:(NSNotification *)note{[self updateUIRefreshTimer];[self refresh:nil];}
- (void)windowDidMiniaturize:(NSNotification *)note{[self updateUIRefreshTimer];}
- (void)windowDidDeminiaturize:(NSNotification *)note{[self updateUIRefreshTimer];[self refresh:nil];}
- (void)menuWillOpen:(NSMenu *)menu{self.menuTracking=YES;self.menuStatus.title=@"正在读取最新状态…";self.menuFull.enabled=NO;self.menuStop.enabled=NO;[self updateUIRefreshTimer];neroRefresh();}
- (void)menuDidClose:(NSMenu *)menu{self.menuTracking=NO;[self updateUIRefreshTimer];}

- (void)dirty:(NSControl *)sender{if(sender.tag==1)self.chargeDirty=YES;if(sender.tag==10)self.thermalDirty=YES;}
@end

void neroRun(void){@autoreleasepool{[NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];neroController=[[NeroController alloc]init];NSApp.delegate=neroController;[NSApp run];}}
void neroRenderJSON(const char *text,int connected){@autoreleasepool{NSData *data=[@(text) dataUsingEncoding:NSUTF8StringEncoding];NSDictionary *value=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];if(![value isKindOfClass:NSDictionary.class])return;dispatch_async(dispatch_get_main_queue(),^{[neroController render:value connected:connected!=0];});}}
void neroActionFinished(int action,int ok,const char *message){@autoreleasepool{NSString *text=@(message);dispatch_async(dispatch_get_main_queue(),^{neroController.feedback.stringValue=text;if(ok&&action==1)neroController.chargeDirty=NO;if(ok&&action==10)neroController.thermalDirty=NO;});}}
void neroHistoryJSON(const char *text){@autoreleasepool{NSData *data=[@(text)dataUsingEncoding:NSUTF8StringEncoding];NSDictionary *value=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];dispatch_async(dispatch_get_main_queue(),^{NSArray *p=value[@"points"];if([p isKindOfClass:NSArray.class]){NSMutableArray *dated=[NSMutableArray arrayWithCapacity:p.count];for(NSDictionary *raw in p){NSMutableDictionary *point=[raw mutableCopy];NSDate *date=NeroDate(raw[@"at"]);if(date)point[@"_date"]=date;[dated addObject:point];}neroController.chart.points=dated;neroController.chart.needsDisplay=YES;NeroValue(neroController,@"history-note",[NSString stringWithFormat:@"本机共显示 %lu 个真实采样点 · 每 30 秒记录 · 保存 30 天",(unsigned long)p.count]);}});}}
