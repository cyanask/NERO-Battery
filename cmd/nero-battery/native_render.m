// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"

static NSString *phaseName(NSString *phase){NSDictionary *m=@{@"Idle":@"尚未开始校准",@"DischargeToThreshold":@"正在放电到目标",@"ChargeToFull":@"正在充到 100%",@"HoldAfterFull":@"正在满电保持",@"DischargeAfterHold":@"正在回到原上限",@"RestoreAndFinish":@"正在恢复原设置",@"Error":@"校准需要处理"};return m[phase]?:@"状态不可用";}
static NSString *modeName(NSString *mode){if([mode isEqualToString:@"automatic"])return @"自动";if([mode isEqualToString:@"low-power"])return @"低电量";return @"—";}
static NSString *ruleName(NSString *kind){return @{@"limit":@"设置上限",@"full":@"临时充满",@"cancel-full":@"取消临时充满",@"discharge":@"手动放电",@"stop-discharge":@"停止放电",@"calibrate":@"校准",@"power-automatic":@"自动电源模式",@"power-low":@"低电量模式",@"awake-30":@"保持唤醒 30 分钟",@"awake-60":@"保持唤醒 60 分钟",@"awake-180":@"保持唤醒 180 分钟",@"sleep":@"恢复正常睡眠"}[kind]?:@"任务";}
static void chooseNumber(NSPopUpButton *popup,NSNumber *number){if(!number)return;for(NSMenuItem *item in popup.itemArray)if(item.title.doubleValue==number.doubleValue){[popup selectItem:item];return;}}

@implementation NeroController(Render)
- (void)render:(NSDictionary *)j connected:(BOOL)connected{
 if(self.window.visible&&self.selectedPanel==0)NeroRenderUSBPower(self);
 self.connected=connected;self.lastStatus=j;
 NSDictionary *battery=NeroGet(j,@"observation.battery")?:@{},*power=NeroGet(j,@"observation.power")?:@{};
 NSInteger upper=[j[@"limit"]integerValue],lower=[j[@"lower_limit"]integerValue],target=[j[@"charge_target"]integerValue];
 BOOL plugged=[j[@"plugged_in"]boolValue];BOOL draining=NeroDate(j[@"discharge_until"])!=nil,hot=[NeroGet(j,@"safety.paused")boolValue];
 NSString *phase=NeroGet(j,@"calibration.phase")?:@"Idle";BOOL calibrating=connected&&![phase isEqualToString:@"Idle"],paused=[NeroGet(j,@"calibration.paused")boolValue],calError=[phase isEqualToString:@"Error"];
 NeroValue(self,@"step-one",calibrating?[NSString stringWithFormat:@"%@%%",NeroGet(j,@"calibration_state.threshold")]:self.calThreshold.titleOfSelectedItem);NeroValue(self,@"step-two",@"100%");NeroValue(self,@"step-three",calibrating?[NSString stringWithFormat:@"%@ 分钟",NeroGet(j,@"calibration_state.holdMinutes")]:self.calHold.titleOfSelectedItem);NeroValue(self,@"step-four",[NSString stringWithFormat:@"%ld%%",upper]);
 NSString *charge=NeroMetricString(battery[@"reported_charge_percent"],@"%",0);NeroValue(self,@"charge",charge);
 NSString *state=connected?[NSString stringWithFormat:@"限充 %ld%% · 已接管",upper]:@"控制未就绪 · 当前仅查看";
 if(connected&&target>0)state=@"临时充满中";if(connected&&draining)state=@"手动放电中";if(connected&&calibrating)state=paused?@"校准已暂停":phaseName(phase);if(connected&&hot)state=@"热保护已暂停充电与放电";
 self.connection.stringValue=connected?@"后台已接管":@"后台未连接";self.connection.textColor=connected?NSColor.secondaryLabelColor:NSColor.systemRedColor;NeroPrimaryTitle(self.quickLimit,connected?[NSString stringWithFormat:@"调整上限 · %ld%%",upper]:@"调整上限");self.statusItem.button.toolTip=@"电池控制 · 点击查看最新状态";self.menuStatus.title=[NSString stringWithFormat:@"%@   ·   %@",charge,state];self.menuStatus.enabled=NO;if(!connected)self.feedback.stringValue=@"后台未连接，充电控制不可用；当前仅显示本机读数。";else if([self.feedback.stringValue hasPrefix:@"后台未连接"])self.feedback.stringValue=@"";
 NSNumber *watts=NeroNumber(power[@"battery_watts"]);NSString *flow=@"电池功率不可用";
 if(watts){if(watts.doubleValue>.5)flow=[NSString stringWithFormat:@"电池充入 %.1f W",watts.doubleValue];else if(watts.doubleValue<-.5)flow=[NSString stringWithFormat:@"电池供电 %.1f W",-watts.doubleValue];else flow=@"电池净功率接近零";}
 self.gauge.progress=[battery[@"reported_charge_percent"]doubleValue];self.gauge.needsDisplay=YES;NeroValue(self,@"flow",flow);NeroValue(self,@"temperature",NeroMetricString(NeroGet(j,@"safety.temperature_c"),@"°C",1));NeroValue(self,@"health",NeroMetricString(battery[@"raw_max_over_design_percent"],@"%",1));NeroValue(self,@"cycles",NeroMetricString(battery[@"cycle_count"],@" 次",0));
 NeroValue(self,@"battery-power",NeroMetricString(power[@"battery_watts"],@" W",1));NeroValue(self,@"adapter-power",NeroMetricString(power[@"adapter_watts"],@" W",1));NeroValue(self,@"system-power",NeroMetricString(power[@"system_watts"],@" W",1));
 NSNumber *current=NeroNumber(battery[@"raw_current_capacity_mah"]),*maximum=NeroNumber(battery[@"raw_max_capacity_mah"]);NeroValue(self,@"capacity",current&&maximum?[NSString stringWithFormat:@"%@ / %@ mAh",current,maximum]:@"—");
 NSNumber *mv=NeroNumber(battery[@"voltage_mv"]);NeroValue(self,@"voltage",mv?[NSString stringWithFormat:@"%.2f V",mv.doubleValue/1000]:@"—");NeroValue(self,@"rated-adapter",NeroMetricString(power[@"adapter_rated_watts"],@" W",0));
 NeroValue(self,@"design-capacity",NeroMetricString(battery[@"design_capacity_mah"],@" mAh",0));NeroValue(self,@"battery-current",NeroMetricString(battery[@"current_ma"],@" mA",0));
 NSNumber *adapterV=NeroNumber(power[@"adapter_voltage_v"]),*adapterA=NeroNumber(power[@"adapter_current_a"]);NeroValue(self,@"adapter-electrical",adapterV&&adapterA?[NSString stringWithFormat:@"%.1f V / %.2f A",adapterV.doubleValue,adapterA.doubleValue]:@"—");
 NSString *strategy=connected?[NSString stringWithFormat:@"充电范围：%ld%% → %ld%%；充电开关：%@；适配器输入：%@。",lower,upper,[j[@"charging_enabled"]boolValue]?@"允许":@"暂停",[j[@"adapter_enabled"]boolValue]?@"启用":@"暂停"]:@"后台未连接，当前只显示本机电池读数。";
 NeroValue(self,@"strategy",strategy);NeroValue(self,@"dashboard-safety",NeroGet(j,@"safety.reason")?:@"等待后台保护状态");
 NSString *historyError=j[@"history_error"];NSDate *historyAt=NeroDate(j[@"history_at"]);NSDateFormatter *clock=[[NSDateFormatter alloc]init];clock.dateFormat=@"HH:mm:ss";
 NeroValue(self,@"history-status",historyError.length?@"历史采集暂未完成，请检查本机存储空间。":(historyAt?[NSString stringWithFormat:@"最近保存：%@ · 数据位于本机受保护目录",[clock stringFromDate:historyAt]]:@"等待首个本机历史采样。"));
 if(!self.loaded||!self.chargeDirty){chooseNumber(self.limit,@(upper));NSInteger gap=upper-lower;NSArray *gaps=@[@2,@3,@5,@10];NSUInteger index=[gaps indexOfObject:@(gap)];if(index!=NSNotFound)[self.gap selectItemAtIndex:index];}
 NeroValue(self,@"band-caption",connected?[NSString stringWithFormat:@"达到 %ld%% 停止；低于 %ld%% 恢复。系统功率读数可能滞后于开关。",upper,lower]:@"当前只能查看电池数据。");
 if(!self.loaded||!self.thermalDirty){self.thermalEnabled.state=[NeroGet(j,@"preferences.thermal.enabled")boolValue]?NSControlStateValueOn:NSControlStateValueOff;chooseNumber(self.thermalPause,NeroGet(j,@"preferences.thermal.pause_c"));chooseNumber(self.thermalResume,NeroGet(j,@"preferences.thermal.resume_c"));}
 NeroValue(self,@"thermal-state",NeroGet(j,@"safety.reason")?:@"等待温度与保护状态");
 NSArray *pressureNames=@[@"正常",@"偏高",@"较高",@"严重"];
 NSNumber *pressureValue=NeroNumber(NeroGet(j,@"safety.system_pressure"));NSInteger pressure=pressureValue?pressureValue.integerValue:-1;NSString *pressureName=pressure>=0&&pressure<pressureNames.count?pressureNames[pressure]:@"不可用";
 NeroValue(self,@"thermal-source",[NSString stringWithFormat:@"电池传感器：%@ · 系统热压力：%@。温度缺失时，已启用的保护会暂停受控充放电。",NeroGet(j,@"safety.sensor")?:@"不可用",pressureName]);
 NeroValue(self,@"calibration-phase",!connected?@"后台状态不可用":calibrating&&paused?[NSString stringWithFormat:@"%@ · 已暂停",phaseName(phase)]:phaseName(phase));
 NSString *detail=@"尚未执行校准。开始后可随时暂停或取消。";
 if(calibrating){detail=paused?@"已恢复外接电源并暂停充电。确认电量、温度与连接正常后，可继续或取消。":@"请保持连接电源并观察当前阶段。";if([phase isEqualToString:@"HoldAfterFull"]&&!paused)detail=[NSString stringWithFormat:@"满电保持剩余约 %.0f 分钟。",ceil([NeroGet(j,@"calibration.remainingHoldSeconds")doubleValue]/60)];if(calError)detail=@"流程已停止。原设置快照保留，请先取消并恢复原设置。";}
 if(!connected)detail=@"连接后台后可查看校准状态。";NeroValue(self,@"calibration-detail",detail);
 self.limit.enabled=self.gap.enabled=connected&&!calibrating;self.thermalEnabled.enabled=self.thermalPause.enabled=self.thermalResume.enabled=connected;
 NSDictionary *enabled=@{@1:@(connected&&!calibrating),@2:@(connected&&plugged&&!hot&&!target&&!draining&&!calibrating),@3:@(connected&&target>0),@4:@(connected&&plugged&&!hot&&!target&&!draining&&!calibrating),@5:@(connected&&draining),@10:@(connected),@11:@(connected&&plugged&&!hot&&!target&&!draining&&!calibrating),@12:@(connected&&calibrating&&!paused&&!calError),@13:@(connected&&plugged&&calibrating&&paused&&!calError&&!hot),@14:@(connected&&calibrating),@20:@(connected),@21:@(connected&&[NeroGet(j,@"power_mode.supported")boolValue]),@22:@(connected)};
 for(NSNumber *key in enabled){self.controls[key].enabled=[enabled[key]boolValue];}self.quickLimit.enabled=[enabled[@1]boolValue];self.limitApply.enabled=self.quickLimit.enabled;self.menuFull.enabled=[enabled[@2]boolValue];self.menuStop.enabled=[enabled[@5]boolValue];
 self.calThreshold.enabled=self.calHold.enabled=connected&&!calibrating;
 if(calibrating){chooseNumber(self.calThreshold,NeroGet(j,@"calibration_state.threshold"));chooseNumber(self.calHold,NeroGet(j,@"calibration_state.holdMinutes"));}
 NSArray *rules=NeroGet(j,@"preferences.schedules");if(![rules isKindOfClass:NSArray.class])rules=@[];
 if(!self.loaded||![self.rules isEqualToArray:rules]){
  self.rules=rules;for(NSView *v in[self.ruleList.arrangedSubviews copy]){[self.ruleList removeArrangedSubview:v];[v removeFromSuperview];}
  if(!rules.count)[self.ruleList addArrangedSubview:NeroLabel(@"未添加定时任务。",13,NSFontWeightRegular)];
  NSDateFormatter *date=[[NSDateFormatter alloc]init];date.dateFormat=@"MM-dd HH:mm";
  for(NSDictionary *rule in rules){
   NSString *days=rule[@"once_at"]?@"仅一次":[rule[@"days"]isEqualToString:@"*"]?@"每天":[rule[@"days"]isEqualToString:@"1-5"]?@"工作日":[rule[@"days"]isEqualToString:@"0,6"]?@"周末":[@"星期 " stringByAppendingString:rule[@"days"]];NSString *kind=ruleName(rule[@"action"]);if([rule[@"action"]isEqualToString:@"limit"])kind=[NSString stringWithFormat:@"限充 %@%%",rule[@"limit"]];
   NSString *receipt=rule[@"result"]?:@"尚未执行";NSDate *last=NeroDate(rule[@"last_run"]);if(last)receipt=[NSString stringWithFormat:@"%@ · %@",[date stringFromDate:last],receipt];
   NSDate *next=NeroDate(rule[@"next_run"]);if(next)receipt=[receipt stringByAppendingFormat:@" · 下次 %@",[date stringFromDate:next]];if([rule[@"name"]length])kind=[NSString stringWithFormat:@"%@ · %@",rule[@"name"],kind];
   NSTextField *label=NeroLabel([NSString stringWithFormat:@"%@ %02ld:%02ld · %@\n%@",days,[rule[@"hour"]integerValue],[rule[@"minute"]integerValue],kind,receipt],12,NSFontWeightRegular);
   NSButton *toggle=[NSButton buttonWithTitle:[rule[@"enabled"]boolValue]?@"停用":@"启用" target:self action:@selector(toggleRule:)];toggle.identifier=rule[@"id"];toggle.enabled=connected;
   NSButton *remove=[NSButton buttonWithTitle:@"移除" target:self action:@selector(removeRule:)];remove.identifier=rule[@"id"];remove.enabled=connected;
   NSButton *edit=[NSButton buttonWithTitle:@"编辑" target:self action:@selector(editRule:)];edit.identifier=rule[@"id"];edit.enabled=connected;NSStackView *row=NeroRow(@[label,edit,toggle,remove]);[self.ruleList addArrangedSubview:row];NeroFullWidth(row,self.ruleList);[label setContentCompressionResistancePriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];
  }
 }
 NeroValue(self,@"cpu-w",NeroMetricString(NeroGet(j,@"energy.cpu_w"),@" W",2));NeroValue(self,@"gpu-w",NeroMetricString(NeroGet(j,@"energy.gpu_w"),@" W",2));NeroValue(self,@"ane-w",NeroMetricString(NeroGet(j,@"energy.ane_w"),@" W",2));
 NSDate *sample=NeroDate(NeroGet(j,@"energy.at"));NSString *note=NeroGet(j,@"energy.message")?:@"尚未采样。";if(sample)note=[NSString stringWithFormat:@"%@ · %@",[clock stringFromDate:sample],note];NeroValue(self,@"energy-note",note);
 NSArray *apps=NeroGet(j,@"energy.apps");if(![apps isKindOfClass:NSArray.class])apps=@[];
 if(![self.energyApps isEqualToArray:apps]){self.energyApps=apps;[self.energyTable reloadData];}
 NeroValue(self,@"energy-apps",apps.count?@"按能耗影响排序；缺失能耗值的项目排在后面。":(sample?@"当前采样未提供可识别的应用数据。":@"点击上方采样按钮，读取当前应用能耗。"));
 NeroValue(self,@"power-current",[NSString stringWithFormat:@"接电：%@ · 使用电池：%@",modeName(NeroGet(j,@"power_mode.ac")),modeName(NeroGet(j,@"power_mode.battery"))]);
 NeroValue(self,@"power-support",NeroGet(j,@"power_mode.message")?:@"等待本机电源模式支持信息。");
 if(!self.loaded){self.sleepStopCharge.state=[NeroGet(j,@"preferences.sleep.stop_charging")boolValue];self.sleepUntilLimit.state=[NeroGet(j,@"preferences.sleep.until_limit")boolValue];self.sleepPauseLid.state=[NeroGet(j,@"preferences.sleep.pause_on_lid")boolValue];chooseNumber(self.sleepMaximum,NeroGet(j,@"preferences.sleep.max_minutes"));chooseNumber(self.sleepFloor,NeroGet(j,@"preferences.sleep.floor"));}
 NSString *sleep=NeroGet(j,@"sleep.reason")?:@"正常睡眠";NSDate *end=NeroDate(NeroGet(j,@"sleep.until"));if([NeroGet(j,@"sleep.active")boolValue]&&end)sleep=[sleep stringByAppendingFormat:@" · 最晚 %@ 结束",[clock stringFromDate:end]];NeroValue(self,@"sleep-state",sleep);self.controls[@40].enabled=connected;self.controls[@41].enabled=connected&&plugged;self.controls[@42].enabled=connected&&([NeroGet(j,@"sleep.active")boolValue]||[NeroGet(j,@"sleep.release_pending")boolValue]);self.loaded=YES;
}
@end
