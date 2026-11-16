// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"

static BOOL confirm(NSString *title,NSString *body,NSString *accept){
 NSAlert *alert=[[NSAlert alloc]init];alert.messageText=title;alert.informativeText=body;[alert addButtonWithTitle:accept];[alert addButtonWithTitle:@"取消"];return[alert runModal]==NSAlertFirstButtonReturn;
}
@implementation NeroController(Actions)
- (void)showLimitPopover:(id)sender{
 if(self.limitPopover.shown){[self.limitPopover performClose:sender];return;}
 if(!self.quickLimit.enabled)return;
 if(!self.limitPopover){
  self.limitPopover=NSPopover.new;self.limitPopover.behavior=NSPopoverBehaviorTransient;
  NSViewController *content=NSViewController.new;content.view=[[NSView alloc]initWithFrame:NSMakeRect(0,0,286,212)];
  NSStackView *stack=NSStackView.new;stack.orientation=NSUserInterfaceLayoutOrientationVertical;stack.alignment=NSLayoutAttributeLeading;stack.spacing=8;stack.translatesAutoresizingMaskIntoConstraints=NO;[content.view addSubview:stack];
  [NSLayoutConstraint activateConstraints:@[[stack.leadingAnchor constraintEqualToAnchor:content.view.leadingAnchor constant:16],[stack.trailingAnchor constraintEqualToAnchor:content.view.trailingAnchor constant:-16],[stack.topAnchor constraintEqualToAnchor:content.view.topAnchor constant:16],[stack.bottomAnchor constraintLessThanOrEqualToAnchor:content.view.bottomAnchor constant:-16]]];
  self.limitPreview=NeroLabel(@"95%",26,NSFontWeightMedium);self.limitPreview.font=[NSFont monospacedDigitSystemFontOfSize:26 weight:NSFontWeightMedium];NSStackView *heading=NeroRow(@[NeroLabel(@"充电上限",13,NSFontWeightSemibold),self.limitPreview]);heading.distribution=NSStackViewDistributionEqualSpacing;[stack addArrangedSubview:heading];NeroFullWidth(heading,stack);
  self.limitSlider=[NSSlider sliderWithValue:95 minValue:50 maxValue:100 target:self action:@selector(previewLimit:)];self.limitSlider.trackFillColor=NeroAccent();self.limitSlider.numberOfTickMarks=11;self.limitSlider.allowsTickMarkValuesOnly=YES;self.limitSlider.continuous=YES;self.limitSlider.accessibilityLabel=@"充电上限草稿，50% 至 100%，每档 5%";[stack addArrangedSubview:self.limitSlider];NeroFullWidth(self.limitSlider,stack);
  NSMutableArray *presets=NSMutableArray.new;for(NSNumber *number in @[@80,@85,@90,@95]){NSButton *b=[NSButton buttonWithTitle:[NSString stringWithFormat:@"%@%%",number] target:self action:@selector(presetLimit:)];b.tag=number.integerValue;[presets addObject:b];}self.limitPresets=presets;NSStackView *row=NeroRow(presets);row.distribution=NSStackViewDistributionFillEqually;[stack addArrangedSubview:row];NeroFullWidth(row,stack);
  self.limitHint=NeroLabel(@"",11,NSFontWeightRegular);self.limitHint.textColor=NSColor.secondaryLabelColor;[stack addArrangedSubview:self.limitHint];NeroFullWidth(self.limitHint,stack);
  NSButton *cancel=[NSButton buttonWithTitle:@"取消" target:self.limitPopover action:@selector(performClose:)];self.limitApply=[NeroPrimaryButton buttonWithTitle:@"应用上限" target:self action:@selector(applyQuickLimit:)];NeroPrimaryTitle(self.limitApply,@"应用上限");[self.limitApply.heightAnchor constraintEqualToConstant:30].active=YES;self.limitApply.keyEquivalent=@"\r";NSStackView *actions=NeroRow(@[cancel,self.limitApply]);actions.distribution=NSStackViewDistributionFillEqually;[stack addArrangedSubview:actions];NeroFullWidth(actions,stack);
  self.limitPopover.contentViewController=content;
 }
 self.limitSlider.integerValue=[self.lastStatus[@"limit"]integerValue];self.limitApply.enabled=self.quickLimit.enabled;[self previewLimit:nil];NSStackView *stack=(NSStackView *)self.limitPopover.contentViewController.view.subviews.firstObject;[self.limitPopover.contentViewController.view layoutSubtreeIfNeeded];self.limitPopover.contentSize=NSMakeSize(286,ceil(stack.fittingSize.height)+32);self.limitPopover.animates=!NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;[self.limitPopover showRelativeToRect:self.quickLimit.bounds ofView:self.quickLimit preferredEdge:NSRectEdgeMinY];
}
- (void)previewLimit:(id)sender{
 NSInteger upper=MAX(50,MIN(100,lround(self.limitSlider.doubleValue/5)*5));self.limitSlider.integerValue=upper;self.limitPreview.stringValue=[NSString stringWithFormat:@"%ld%%",upper];NSInteger gap=[self.lastStatus[@"limit"]integerValue]-[self.lastStatus[@"lower_limit"]integerValue];if(![@[@2,@3,@5,@10]containsObject:@(gap)])gap=2;self.limitHint.stringValue=[NSString stringWithFormat:@"低于 %ld%% 恢复充电 · 应用后生效",upper-gap];
 for(NSButton *button in self.limitPresets){BOOL selected=button.tag==upper;button.bezelColor=selected?[NSColor colorWithSRGBRed:.88 green:.95 blue:.91 alpha:1]:nil;NSMutableParagraphStyle *p=NSMutableParagraphStyle.new;p.alignment=NSTextAlignmentCenter;button.attributedTitle=[[NSAttributedString alloc]initWithString:[NSString stringWithFormat:@"%ld%%",button.tag] attributes:@{NSFontAttributeName:[NSFont systemFontOfSize:12 weight:selected?NSFontWeightSemibold:NSFontWeightRegular],NSForegroundColorAttributeName:selected?[NSColor colorWithSRGBRed:.07 green:.40 blue:.29 alpha:1]:NSColor.labelColor,NSParagraphStyleAttributeName:p}];}
}
- (void)presetLimit:(NSButton *)sender{self.limitSlider.integerValue=sender.tag;[self previewLimit:nil];}
- (void)applyQuickLimit:(id)sender{
 if(!self.connected||!self.controls[@1].enabled){self.feedback.stringValue=@"后台控制未就绪，充电上限尚未更改。";return;}
 NSInteger upper=self.limitSlider.integerValue;NSString *title=[NSString stringWithFormat:@"%ld%%",upper];if([self.limit indexOfItemWithTitle:title]<0)return;
 NSInteger gap=[self.lastStatus[@"limit"]integerValue]-[self.lastStatus[@"lower_limit"]integerValue];NSUInteger index=[@[@2,@3,@5,@10]indexOfObject:@(gap)];[self.limit selectItemWithTitle:title];[self.gap selectItemAtIndex:index==NSNotFound?0:index];self.chargeDirty=YES;[self.limitPopover performClose:sender];[self act:self.controls[@1]];
}
- (void)act:(id)sender{
 NSInteger action=[sender tag];id payload=nil;
 if(action==40){payload=@{@"stop_charging":@((BOOL)(self.sleepStopCharge.state==NSControlStateValueOn)),@"until_limit":@((BOOL)(self.sleepUntilLimit.state==NSControlStateValueOn)),@"pause_on_lid":@((BOOL)(self.sleepPauseLid.state==NSControlStateValueOn)),@"max_minutes":@(self.sleepMaximum.titleOfSelectedItem.integerValue),@"floor":@(self.sleepFloor.titleOfSelectedItem.integerValue)};}
 if(action==41)payload=@(self.sleepDuration.titleOfSelectedItem.integerValue);
 if(action==1){NSInteger gaps[]={2,3,5,10};NSInteger upper=self.limit.titleOfSelectedItem.integerValue;payload=@{@"upper":@(upper),@"lower":@(upper-gaps[self.gap.indexOfSelectedItem])};}
 if(action==4&&!confirm(@"开始使用电池供电？",@"最长持续 1 小时。电量到 30%、进入睡眠或触发热保护时停止；可随时点击“停止放电”。请保持屏幕打开。",@"开始手动放电"))return;
 if(action==10){
  BOOL enabled=self.thermalEnabled.state==NSControlStateValueOn;
  if(!enabled&&[NeroGet(self.lastStatus,@"preferences.thermal.enabled")boolValue]&&!confirm(@"关闭本软件的附加热保护？",@"关闭后将按充电范围运行，macOS 自带的安全保护继续有效。",@"关闭附加热保护"))return;
  payload=@{@"enabled":@(enabled),@"pause_c":@(self.thermalPause.titleOfSelectedItem.doubleValue),@"resume_c":@(self.thermalResume.titleOfSelectedItem.doubleValue)};
 }
 if(action==11){NSInteger threshold=self.calThreshold.titleOfSelectedItem.integerValue,hold=self.calHold.titleOfSelectedItem.integerValue;
  if(!confirm(@"开始长时间校准流程？",[NSString stringWithFormat:@"将放电到 %ld%%、充满并保持 %ld 分钟，再回到开始前的上限。请连接电源、保持屏幕打开并自行观察。可随时暂停或取消。",threshold,hold],@"开始校准"))return;
  payload=@{@"threshold":@(threshold),@"hold_minutes":@(hold)};
 }
 if(action==21){NSString *mode=self.powerMode.indexOfSelectedItem==1?@"low-power":@"automatic";if(!confirm(@"应用系统电源模式？",@"将同时更改接电和使用电池时的模式。",@"应用"))return;payload=mode;}
 self.feedback.stringValue=action==22?@"正在进行约 1 秒的本机能耗采样…":@"正在应用…";
 NeroSend(action,payload);
}
- (void)historyChanged:(id)sender{
 self.chart.metric=@[@"charge",@"temperature_c",@"system_w"][self.historyMetric.selectedSegment];self.chart.needsDisplay=YES;self.historyRefreshAt=NSDate.date;
 NeroSend(30,@[@"1h",@"24h",@"7d",@"30d"][self.historyRange.selectedSegment]);
}
- (void)openShortcuts:(id)sender{[NSWorkspace.sharedWorkspace openURL:[NSURL fileURLWithPath:@"/System/Applications/Shortcuts.app"]];}
- (void)ruleRecurrence:(id)sender{BOOL once=self.ruleDays.indexOfSelectedItem==4;self.ruleTime.datePickerElements=once?(NSDatePickerElementFlagYearMonthDay|NSDatePickerElementFlagHourMinute):NSDatePickerElementFlagHourMinute;for(NSButton *b in self.ruleWeekdays)b.enabled=self.ruleDays.indexOfSelectedItem==3;}
- (void)editRule:(NSButton *)sender{
 for(NSDictionary *r in self.rules)if([r[@"id"]isEqualToString:sender.identifier]){
 self.editingRule=r[@"id"];self.ruleName.stringValue=r[@"name"]?:@"";NSArray *a=@[@"limit",@"full",@"cancel-full",@"stop-discharge",@"discharge",@"calibrate",@"power-automatic",@"power-low",@"awake-30",@"awake-60",@"awake-180",@"sleep"];NSUInteger index=[a indexOfObject:r[@"action"]];if(index!=NSNotFound)[self.ruleAction selectItemAtIndex:index];
 NSString *days=r[@"days"];NSInteger repeat=r[@"once_at"]?4:[days isEqualToString:@"*"]?0:[days isEqualToString:@"1-5"]?1:[days isEqualToString:@"0,6"]?2:3;[self.ruleDays selectItemAtIndex:repeat];for(NSInteger i=0;i<7;i++)self.ruleWeekdays[i].state=[[days componentsSeparatedByString:@","]containsObject:[@(i)stringValue]]?NSControlStateValueOn:NSControlStateValueOff;
 NSDateComponents *c=[NSCalendar.currentCalendar components:NSCalendarUnitYear|NSCalendarUnitMonth|NSCalendarUnitDay fromDate:NSDate.date];c.hour=[r[@"hour"]integerValue];c.minute=[r[@"minute"]integerValue];self.ruleTime.dateValue=NeroDate(r[@"once_at"])?:[NSCalendar.currentCalendar dateFromComponents:c];for(NSMenuItem *item in self.ruleLimit.itemArray)if(item.title.integerValue==[r[@"limit"]integerValue])[self.ruleLimit selectItem:item];self.ruleSave.title=@"保存任务…";[self ruleRecurrence:nil];self.feedback.stringValue=@"正在编辑任务，修改后点保存。";break;
 }
}
- (void)addRule:(id)sender{
 if(!self.editingRule&&self.rules.count>=16){self.feedback.stringValue=@"最多保存 16 项定时任务。";return;}
 NSArray *a=@[@"limit",@"full",@"cancel-full",@"stop-discharge",@"discharge",@"calibrate",@"power-automatic",@"power-low",@"awake-30",@"awake-60",@"awake-180",@"sleep"];NSString *kind=a[self.ruleAction.indexOfSelectedItem];NSString *days=@"*";NSInteger repeat=self.ruleDays.indexOfSelectedItem;
 if(repeat==1)days=@"1-5";if(repeat==2)days=@"0,6";if(repeat==3){NSMutableArray *d=NSMutableArray.new;for(NSInteger i=0;i<7;i++)if(self.ruleWeekdays[i].state==NSControlStateValueOn)[d addObject:[@(i)stringValue]];if(!d.count){self.feedback.stringValue=@"请选择至少一个星期。";return;}days=[d componentsJoinedByString:@","];}
 if(repeat==4&&[self.ruleTime.dateValue timeIntervalSinceNow]<=0){self.feedback.stringValue=@"一次性任务请选择未来时间。";return;}
 if(!confirm(@"保存并启用定时任务？",[NSString stringWithFormat:@"将按所选时间执行“%@”。%@",self.ruleAction.titleOfSelectedItem,([kind isEqualToString:@"discharge"]||[kind isEqualToString:@"calibrate"])?@"任务包含实际放电，请届时自行观察。":@"不会唤醒睡眠中的电脑。"],@"保存任务"))return;
 NSDateComponents *c=[NSCalendar.currentCalendar components:NSCalendarUnitHour|NSCalendarUnitMinute fromDate:self.ruleTime.dateValue];NSMutableDictionary *rule=[@{@"id":self.editingRule?:NSUUID.UUID.UUIDString,@"name":self.ruleName.stringValue,@"enabled":@YES,@"action":kind,@"hour":@(c.hour),@"minute":@(c.minute),@"days":days,@"limit":@(self.ruleLimit.titleOfSelectedItem.integerValue)}mutableCopy];if(repeat==4){NSISO8601DateFormatter *fmt=NSISO8601DateFormatter.new;rule[@"once_at"]=[fmt stringFromDate:self.ruleTime.dateValue];}
 NSMutableArray *rules=NSMutableArray.new;for(NSDictionary *r in self.rules)if(![r[@"id"]isEqualToString:self.editingRule])[rules addObject:r];[rules addObject:rule];NeroSend(20,rules);self.editingRule=nil;self.ruleSave.title=@"添加任务…";self.ruleName.stringValue=@"";
}
- (void)toggleRule:(NSButton *)sender{
 NSMutableArray *rules=[NSMutableArray array];for(NSDictionary *old in self.rules){NSMutableDictionary *r=[old mutableCopy];if([old[@"id"]isEqualToString:sender.identifier])r[@"enabled"]=@(![old[@"enabled"]boolValue]);[rules addObject:r];}NeroSend(20,rules);
}
- (void)removeRule:(NSButton *)sender{
 if(!confirm(@"移除此定时任务？",@"移除后不会再执行；可以重新添加。",@"移除"))return;
 NSMutableArray *rules=[NSMutableArray array];for(NSDictionary *r in self.rules)if(![r[@"id"]isEqualToString:sender.identifier])[rules addObject:r];NeroSend(20,rules);
}
@end
