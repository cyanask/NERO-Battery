// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"

static NSTextField *valueLabel(NeroController *o,NSString *key,CGFloat size){NSTextField *v=NeroLabel(@"—",size,NSFontWeightMedium);if(size>=17)v.font=[NSFont monospacedDigitSystemFontOfSize:size weight:NSFontWeightMedium];o.values[key]=v;return v;}
static NSView *metric(NeroController *o,NSString *key,NSString *title){
 NSBox *box=[[NSBox alloc]init];box.boxType=NSBoxCustom;box.titlePosition=NSNoTitle;box.borderWidth=0;box.borderColor=NSColor.separatorColor;box.fillColor=NSColor.controlBackgroundColor;box.cornerRadius=9;box.contentViewMargins=NSZeroSize;
 NSStackView *s=[[NSStackView alloc]init];s.orientation=NSUserInterfaceLayoutOrientationVertical;s.alignment=NSLayoutAttributeLeading;s.spacing=4;s.translatesAutoresizingMaskIntoConstraints=NO;[box.contentView addSubview:s];
 NSTextField *caption=NeroLabel(title,12,NSFontWeightRegular);caption.textColor=NSColor.secondaryLabelColor;[s addArrangedSubview:caption];[s addArrangedSubview:valueLabel(o,key,17)];
 [NSLayoutConstraint activateConstraints:@[[s.leadingAnchor constraintEqualToAnchor:box.contentView.leadingAnchor constant:14],[s.trailingAnchor constraintLessThanOrEqualToAnchor:box.contentView.trailingAnchor constant:-14],[s.topAnchor constraintEqualToAnchor:box.contentView.topAnchor constant:10],[s.bottomAnchor constraintLessThanOrEqualToAnchor:box.contentView.bottomAnchor constant:-10],[box.heightAnchor constraintEqualToConstant:58]]];return box;
}
// Labelled native form rows keep controls aligned and keyboard-accessible.
static void formRow(NSStackView *parent,NSString *title,NSArray<NSView *> *controls){
 NSTextField *label=NeroLabel(title,13,NSFontWeightMedium);[label.widthAnchor constraintEqualToConstant:116].active=YES;
 NSView *space=NSView.new;[space setContentHuggingPriority:1 forOrientation:NSLayoutConstraintOrientationHorizontal];
 NSMutableArray *items=[NSMutableArray arrayWithObjects:label,space,nil];[items addObjectsFromArray:controls];NSStackView *row=NeroRow(items);[parent addArrangedSubview:row];NeroFullWidth(row,parent);
}
static NSArray *percentItems(NSInteger first,NSInteger last,NSInteger step){NSMutableArray *a=[NSMutableArray array];for(NSInteger i=first;i<=last;i+=step)[a addObject:[NSString stringWithFormat:@"%ld%%",i]];return a;}
static void metricRow(NeroController *o,NSStackView *stack,NSArray *keys,NSArray *titles){NSMutableArray *v=[NSMutableArray array];for(NSInteger i=0;i<keys.count;i++)[v addObject:metric(o,keys[i],titles[i])];NSStackView *r=NeroRow(v);r.distribution=NSStackViewDistributionFillEqually;[stack addArrangedSubview:r];NeroFullWidth(r,stack);}

@implementation NeroController(Panels)
- (void)buildDashboard{
 NSStackView *root=[self newPanel];NeroSection(root,@"总览",nil);NSStackView *s=NeroCard(root);
 self.gauge=NeroGauge.new;[self.gauge.widthAnchor constraintEqualToConstant:76].active=YES;[self.gauge.heightAnchor constraintEqualToConstant:76].active=YES;
 NSStackView *summary=NSStackView.new;summary.orientation=NSUserInterfaceLayoutOrientationVertical;summary.alignment=NSLayoutAttributeLeading;summary.spacing=5;
 [summary addArrangedSubview:valueLabel(self,@"charge",30)];[summary addArrangedSubview:valueLabel(self,@"flow",14)];NSTextField *policy=valueLabel(self,@"strategy",12);policy.textColor=NSColor.secondaryLabelColor;[summary addArrangedSubview:policy];
 NSView *spring=NSView.new;[spring setContentHuggingPriority:1 forOrientation:NSLayoutConstraintOrientationHorizontal];[spring setContentCompressionResistancePriority:1 forOrientation:NSLayoutConstraintOrientationHorizontal];
 self.quickLimit=[NeroPrimaryButton buttonWithTitle:@"调整上限" target:self action:@selector(showLimitPopover:)];self.quickLimit.bezelStyle=NSBezelStyleRounded;self.quickLimit.bezelColor=[NSColor colorWithSRGBRed:.07 green:.40 blue:.29 alpha:1];self.quickLimit.contentTintColor=NSColor.whiteColor;self.quickLimit.font=[NSFont systemFontOfSize:12 weight:NSFontWeightMedium];NSImage *sliderSymbol=[NSImage imageWithSystemSymbolName:@"slider.horizontal.3" accessibilityDescription:nil];self.quickLimit.image=[NSImage imageWithSize:NSMakeSize(14,14) flipped:NO drawingHandler:^BOOL(NSRect rect){[sliderSymbol drawInRect:rect];[NSColor.whiteColor setFill];NSRectFillUsingOperation(rect,NSCompositingOperationSourceIn);return YES;}];NeroPrimaryTitle(self.quickLimit,@"调整上限");self.quickLimit.imagePosition=NSImageLeading;self.quickLimit.accessibilityLabel=@"调整充电上限";[self.quickLimit.heightAnchor constraintEqualToConstant:36].active=YES;
 NSStackView *quick=[NSStackView stackViewWithViews:@[self.quickLimit,self.connection]];quick.orientation=NSUserInterfaceLayoutOrientationVertical;quick.alignment=NSLayoutAttributeTrailing;quick.spacing=7;[quick.widthAnchor constraintEqualToConstant:174].active=YES;[self.quickLimit.widthAnchor constraintEqualToConstant:174].active=YES;
 NSStackView *hero=NeroRow(@[self.gauge,summary,spring,quick]);hero.spacing=14;[s addArrangedSubview:hero];NeroFullWidth(hero,s);
 metricRow(self,root,@[@"temperature",@"health",@"cycles"],@[@"电池温度",@"容量比 · 最大 / 设计",@"循环次数"]);
 s=NeroCard(root);NeroSection(s,@"电力流向",@"功率为当前读数，系统耗电按功率差推算。");
 metricRow(self,s,@[@"adapter-power",@"battery-power",@"system-power",@"usb-output"],@[@"电源输入",@"电池净功率",@"系统耗电 · 推算",@"USB 输出"]);
 [s addArrangedSubview:valueLabel(self,@"usb-ports",12)];NeroFullWidth(self.values[@"usb-ports"],s);
 [s addArrangedSubview:valueLabel(self,@"usb-note",11)];NeroFullWidth(self.values[@"usb-note"],s);
 [s addArrangedSubview:valueLabel(self,@"dashboard-safety",12)];[s addArrangedSubview:valueLabel(self,@"history-status",11)];NeroFullWidth(self.values[@"history-status"],s);
 s=NeroCard(root);NeroSection(s,@"电池与适配器规格",nil);
 metricRow(self,s,@[@"capacity",@"design-capacity",@"rated-adapter"],@[@"当前 / 最大容量",@"设计容量",@"电源标称功率"]);
 metricRow(self,s,@[@"voltage",@"battery-current",@"adapter-electrical"],@[@"电池电压",@"电池电流",@"适配器电压 / 电流"]);
}
- (void)buildCharging{
 NSStackView *root=[self newPanel];NSStackView *s=NeroCard(root);NeroSection(s,@"充电上下限",@"电量达到上限时停止充电，下降到恢复电量时再充。设置在后台重启后保留。");
 self.limit=NeroPopup(percentItems(50,100,5),9);self.gap=NeroPopup(@[@"低 2 个百分点",@"低 3 个百分点",@"低 5 个百分点",@"低 10 个百分点"],0);
 self.limit.target=self;self.limit.action=@selector(dirty:);self.limit.tag=1;self.gap.target=self;self.gap.action=@selector(dirty:);self.gap.tag=1;
 self.limit.accessibilityLabel=@"充电上限";self.gap.accessibilityLabel=@"恢复充电间隔";
 formRow(s,@"充电上限",@[self.limit]);formRow(s,@"恢复间隔",@[self.gap]);formRow(s,@"保存设置",@[NeroButton(self,@"应用充电范围",1)]);
 [s addArrangedSubview:valueLabel(self,@"band-caption",13)];
 s=NeroCard(root);NeroSection(s,@"临时充满",@"暂时充到 100%，完成或取消后恢复原上限。后台重启会取消临时任务。");
 [s addArrangedSubview:NeroRow(@[NeroButton(self,@"临时充到 100%",2),NeroButton(self,@"取消并恢复上限",3)])];
 s=NeroCard(root);NeroSection(s,@"手动放电",@"保持屏幕打开且连接电源。最长 1 小时；电量到 30%、睡眠、热保护或后台重启时停止。");
 [s addArrangedSubview:NeroRow(@[NeroButton(self,@"开始手动放电",4),NeroButton(self,@"停止放电并恢复电源",5)])];
 s=NeroCard(root);NeroSection(s,@"热保护",@"电池温度或系统热压力异常时暂停充电与放电。温度下降到恢复值后，正常限充继续；校准保持暂停，需手动继续。");
 self.thermalEnabled=[NSButton checkboxWithTitle:@"启用附加热保护" target:self action:@selector(dirty:)];self.thermalEnabled.tag=10;[s addArrangedSubview:self.thermalEnabled];
 self.thermalPause=NeroPopup(@[@"35°C",@"37°C",@"40°C",@"42°C",@"45°C"],2);self.thermalResume=NeroPopup(@[@"30°C",@"32°C",@"35°C",@"37°C",@"40°C"],3);
 for(NSPopUpButton *p in @[self.thermalPause,self.thermalResume]){p.target=self;p.action=@selector(dirty:);p.tag=10;}
 formRow(s,@"暂停温度",@[self.thermalPause]);formRow(s,@"恢复温度",@[self.thermalResume,NeroButton(self,@"应用热保护",10)]);
 [s addArrangedSubview:valueLabel(self,@"thermal-state",14)];NeroFullWidth(self.values[@"thermal-state"],s);
 [s addArrangedSubview:valueLabel(self,@"thermal-source",12)];NeroFullWidth(self.values[@"thermal-source"],s);
}
- (void)buildCalibration{
 NSStackView *root=[self newPanel];NSStackView *s=NeroCard(root);NeroSection(s,@"电池校准",@"这是包含充放电的长流程，需要你主动开始。保持连接电源、屏幕打开，并预留足够时间。");
 [s addArrangedSubview:valueLabel(self,@"calibration-phase",23)];
 NSStackView *steps=NeroRow(@[metric(self,@"step-one",@"01 · 放电到目标"),metric(self,@"step-two",@"02 · 充到满电"),metric(self,@"step-three",@"03 · 满电保持"),metric(self,@"step-four",@"04 · 恢复上限")]);steps.distribution=NSStackViewDistributionFillEqually;[s addArrangedSubview:steps];NeroFullWidth(steps,s);
 self.calThreshold=NeroPopup(@[@"15%",@"20%",@"25%",@"30%"],1);self.calHold=NeroPopup(@[@"10 分钟",@"30 分钟",@"60 分钟",@"120 分钟",@"180 分钟"],2);
 formRow(s,@"放电目标",@[self.calThreshold]);formRow(s,@"满电保持",@[self.calHold]);
 [s addArrangedSubview:NeroRow(@[NeroButton(self,@"开始校准…",11),NeroButton(self,@"暂停并恢复外接电源",12)])];
 [s addArrangedSubview:NeroRow(@[NeroButton(self,@"继续校准",13),NeroButton(self,@"取消并恢复原设置",14)])];
 [s addArrangedSubview:valueLabel(self,@"calibration-detail",14)];NeroFullWidth(self.values[@"calibration-detail"],s);
 s=NeroCard(root);NeroSection(s,@"中断与恢复",@"睡眠或后台重启后不会自动继续放电。暂停时恢复外接电源并暂停充电；取消后恢复开始前的充电上下限。恢复状态写入失败时，停止流程并保留原快照。");
 [s addArrangedSubview:NeroLabel(@"校准不会提高电池的物理容量，也不适合频繁执行。",13,NSFontWeightRegular)];
}
- (void)buildHistory{
 NSStackView *root=[self newPanel];NSStackView *s=NeroCard(root);NeroSection(s,@"本机历史",@"只显示安装后采集到的真实数据。没有读数的区间不会补零或生成曲线。");
 self.historyRange=[NSSegmentedControl segmentedControlWithLabels:@[@"1 小时",@"24 小时",@"7 天",@"30 天"] trackingMode:NSSegmentSwitchTrackingSelectOne target:self action:@selector(historyChanged:)];self.historyRange.selectedSegment=0;
 self.historyMetric=[NSSegmentedControl segmentedControlWithLabels:@[@"电量",@"温度",@"系统功率"] trackingMode:NSSegmentSwitchTrackingSelectOne target:self action:@selector(historyChanged:)];self.historyMetric.selectedSegment=0;
 formRow(s,@"查看范围",@[self.historyRange]);formRow(s,@"图表指标",@[self.historyMetric]);
 self.chart=[[NeroChart alloc]init];self.chart.metric=@"charge";self.chart.points=@[];[self.chart.heightAnchor constraintEqualToConstant:290].active=YES;[s addArrangedSubview:self.chart];NeroFullWidth(self.chart,s);
 [s addArrangedSubview:valueLabel(self,@"history-note",13)];NeroFullWidth(self.values[@"history-note"],s);
 [s addArrangedSubview:NeroLabel(@"系统功率按适配器输入功率减去电池净充入功率推算；CPU / GPU / ANE 的系统估算在“能耗与电源”中查看。",12,NSFontWeightRegular)];
}
- (void)buildAutomation{
 NSStackView *root=[self newPanel];NeroSection(root,@"定时与捷径",@"按本机时间执行，睡眠期间不唤醒电脑，也不补执行错过的任务。");NSStackView *s=NeroCard(root);NeroSection(s,@"安排一项任务",nil);
 self.ruleName=[NSTextField textFieldWithString:@""];self.ruleName.placeholderString=@"任务名称（可选）";[s addArrangedSubview:self.ruleName];NeroFullWidth(self.ruleName,s);
 self.ruleAction=NeroPopup(@[@"设置充电上限",@"临时充满",@"取消临时充满",@"停止放电",@"手动放电 · 最长 1 小时",@"校准 · 20% / 60 分钟",@"自动电源模式",@"低电量模式",@"保持唤醒 30 分钟",@"保持唤醒 60 分钟",@"保持唤醒 180 分钟",@"恢复正常睡眠"],0);
 self.ruleDays=NeroPopup(@[@"每天",@"工作日",@"周末",@"指定星期",@"仅一次"],0);self.ruleDays.target=self;self.ruleDays.action=@selector(ruleRecurrence:);
 self.ruleTime=NSDatePicker.new;self.ruleTime.datePickerStyle=NSDatePickerStyleTextFieldAndStepper;self.ruleTime.datePickerElements=NSDatePickerElementFlagHourMinute;self.ruleTime.dateValue=[NSDate dateWithTimeIntervalSinceNow:3600];
 self.ruleLimit=NeroPopup(percentItems(50,100,5),9);formRow(s,@"执行操作",@[self.ruleAction]);formRow(s,@"重复与时间",@[self.ruleDays,self.ruleTime]);
 NSMutableArray *days=NSMutableArray.new;NSArray *dayTitles=@[@"日",@"一",@"二",@"三",@"四",@"五",@"六"];for(NSString *title in dayTitles){NSButton *b=[NSButton checkboxWithTitle:title target:nil action:nil];b.state=NSControlStateValueOn;[days addObject:b];}self.ruleWeekdays=days;[s addArrangedSubview:NeroRow(days)];
 self.ruleSave=[NSButton buttonWithTitle:@"添加任务…" target:self action:@selector(addRule:)];self.controls[@20]=self.ruleSave;formRow(s,@"充电上限",@[self.ruleLimit,self.ruleSave]);[self ruleRecurrence:nil];
 s=NeroCard(root);NeroSection(s,@"已安排",@"可编辑、停用或移除。结果及下次执行时间由本机后台记录。");self.ruleList=NSStackView.new;self.ruleList.orientation=NSUserInterfaceLayoutOrientationVertical;self.ruleList.alignment=NSLayoutAttributeLeading;self.ruleList.spacing=10;[s addArrangedSubview:self.ruleList];NeroFullWidth(self.ruleList,s);
 s=NeroCard(root);NeroSection(s,@"原生快捷指令 · 暂不可用",@"当前系统尚未显示本应用的原生动作，此项未通过验收。定时任务可独立使用。");[s addArrangedSubview:[NSButton buttonWithTitle:@"打开快捷指令" target:self action:@selector(openShortcuts:)]];
}
- (void)buildEnergy{
 NSStackView *root=[self newPanel];NSStackView *s=NeroCard(root);NeroSection(s,@"细分功率",@"使用 macOS powermetrics 单次约 1 秒采样。CPU / GPU / ANE 功率为系统估算；不支持的项目显示为 —。");
 metricRow(self,s,@[@"cpu-w",@"gpu-w",@"ane-w"],@[@"CPU · 估算",@"GPU · 估算",@"ANE · 估算"]);
 [s addArrangedSubview:NeroButton(self,@"采样应用能耗与细分功率",22)];
 [s addArrangedSubview:valueLabel(self,@"energy-note",12)];NeroFullWidth(self.values[@"energy-note"],s);
 s=NeroCard(root);NeroSection(s,@"应用与进程能耗",@"按进程显示，应用的多个子进程不合并。能耗影响是 macOS 的相对指标，不等于瓦数；进程短名仅用于当前显示，不写入历史记录。");
 NSTextField *apps=valueLabel(self,@"energy-apps",12);[s addArrangedSubview:apps];NeroFullWidth(apps,s);
 NSScrollView *list=[[NSScrollView alloc]init];list.hasVerticalScroller=NO;list.hasHorizontalScroller=NO;list.borderType=NSBezelBorder;
 self.energyTable=[[NSTableView alloc]init];self.energyTable.dataSource=self;self.energyTable.usesAlternatingRowBackgroundColors=YES;self.energyTable.rowHeight=23;self.energyTable.columnAutoresizingStyle=NSTableViewFirstColumnOnlyAutoresizingStyle;
 NSArray *ids=@[@"name",@"cpu",@"impact"],*titles=@[@"应用 / 进程",@"CPU（单核100%）",@"能耗影响"];
 for(NSInteger i=0;i<ids.count;i++){NSTableColumn *col=[[NSTableColumn alloc]initWithIdentifier:ids[i]];col.title=titles[i];col.width=i==0?380:150;col.minWidth=i==0?160:120;NSTextFieldCell *cell=[[NSTextFieldCell alloc]init];cell.font=i==0?[NSFont systemFontOfSize:12]:[NSFont monospacedDigitSystemFontOfSize:12 weight:NSFontWeightRegular];cell.alignment=i==0?NSTextAlignmentLeft:NSTextAlignmentRight;cell.lineBreakMode=NSLineBreakByTruncatingTail;cell.textColor=NSColor.labelColor;cell.editable=NO;col.dataCell=cell;[self.energyTable addTableColumn:col];}
 list.documentView=self.energyTable;[list.heightAnchor constraintEqualToConstant:250].active=YES;[s addArrangedSubview:list];NeroFullWidth(list,s);
 s=NeroCard(root);NeroSection(s,@"电源模式",@"更改接电和使用电池时的系统模式。本机支持范围以 macOS 报告为准。");
 [s addArrangedSubview:valueLabel(self,@"power-current",14)];NeroFullWidth(self.values[@"power-current"],s);
 self.powerMode=NeroPopup(@[@"自动",@"低电量模式"],0);formRow(s,@"电源模式",@[self.powerMode,NeroButton(self,@"应用电源模式…",21)]);
 [s addArrangedSubview:valueLabel(self,@"power-support",12)];NeroFullWidth(self.values[@"power-support"],s);
}
@end

@implementation NeroController(SleepPanel)
- (void)buildSleep{
 NSStackView *root=[self newPanel];NeroSection(root,@"睡眠与合盖",@"临时保持唤醒，按电量和电源状态自动结束。");NSStackView *s=NeroCard(root);
 self.sleepStopCharge=[NSButton checkboxWithTitle:@"进入睡眠前暂停充电" target:nil action:nil];self.sleepUntilLimit=[NSButton checkboxWithTitle:@"接电时保持唤醒，充至上限后允许睡眠" target:nil action:nil];self.sleepPauseLid=[NSButton checkboxWithTitle:@"合盖时暂停充电，开盖后恢复原策略" target:nil action:nil];
 for(NSButton *b in @[self.sleepStopCharge,self.sleepUntilLimit,self.sleepPauseLid])[s addArrangedSubview:b];
 self.sleepMaximum=NeroPopup(@[@"30 分钟",@"60 分钟",@"120 分钟",@"240 分钟",@"480 分钟"],2);self.sleepFloor=NeroPopup(@[@"10%",@"15%",@"20%",@"30%",@"40%",@"50%"],2);
 formRow(s,@"最长保持",@[self.sleepMaximum]);formRow(s,@"低电量结束",@[self.sleepFloor]);[s addArrangedSubview:NeroButton(self,@"保存睡眠联动",40)];
 s=NeroCard(root);NeroSection(s,@"临时保持唤醒",@"仅接电时可启动。断电、合盖、低电量、热保护或后台重启时自动结束；显示器仍可休眠。");self.sleepDuration=NeroPopup(@[@"30 分钟",@"60 分钟",@"180 分钟",@"480 分钟"],1);[s addArrangedSubview:NeroRow(@[self.sleepDuration,NeroButton(self,@"开始保持",41),NeroButton(self,@"恢复正常睡眠",42)])];[s addArrangedSubview:valueLabel(self,@"sleep-state",13)];NeroFullWidth(self.values[@"sleep-state"],s);
 s=NeroCard(root);NeroSection(s,@"合盖与唤醒",@"合盖或系统强制睡眠始终交由 macOS 决定。主动放电与校准在睡眠前暂停并恢复外接供电；唤醒后重新应用已保存的充电范围。不会修改系统锁屏、密码要求或全局睡眠配置。");
}
@end
