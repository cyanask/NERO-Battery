// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import <Cocoa/Cocoa.h>

extern void neroRefresh(void);
extern void neroPerform(int action,const char *payload);
extern void neroControlClientActive(int active);
NSDictionary *NeroUSBPowerSnapshot(void);
@class NeroController;
void NeroRenderUSBPower(NeroController *controller);

@interface NeroPrimaryButton:NSButton
@end

@interface NeroGauge:NSView
@property CGFloat progress;
@end

@interface NeroChart : NSView
@property NSArray<NSDictionary *> *points;
@property NSString *metric;
@end

@interface NeroController : NSObject <NSApplicationDelegate,NSWindowDelegate,NSMenuDelegate>
@property NeroGauge *gauge;
@property NSTextField *pageTitle;
@property NSButton *sleepStopCharge;
@property NSButton *sleepUntilLimit;
@property NSButton *sleepPauseLid;
@property NSPopUpButton *sleepDuration;
@property NSPopUpButton *sleepMaximum;
@property NSPopUpButton *sleepFloor;
@property NSDatePicker *ruleTime;
@property NSTextField *ruleName;
@property NSArray<NSButton *> *ruleWeekdays;
@property NSString *editingRule;
@property NSButton *ruleSave;
@property NSWindow *window;
@property NSStatusItem *statusItem;
@property NSMenuItem *menuStatus;
@property NSMenuItem *menuFull;
@property NSMenuItem *menuStop;
@property NSView *host;
@property NSMutableArray<NSScrollView *> *panels;
@property NSMutableArray<NSButton *> *navigation;
@property NSMutableDictionary<NSString *,NSTextField *> *values;
@property NSMutableDictionary<NSNumber *,NSControl *> *controls;
@property NSTextField *connection;
@property NSButton *quickLimit;
@property NSPopover *limitPopover;
@property NSSlider *limitSlider;
@property NSTextField *limitPreview;
@property NSTextField *limitHint;
@property NSButton *limitApply;
@property NSArray<NSButton *> *limitPresets;
@property NSTextField *feedback;
@property NSPopUpButton *limit;
@property NSPopUpButton *gap;
@property NSButton *thermalEnabled;
@property NSPopUpButton *thermalPause;
@property NSPopUpButton *thermalResume;
@property NSPopUpButton *calThreshold;
@property NSPopUpButton *calHold;
@property NSPopUpButton *ruleAction;
@property NSPopUpButton *ruleDays;
@property NSPopUpButton *ruleHour;
@property NSPopUpButton *ruleMinute;
@property NSPopUpButton *ruleLimit;
@property NSStackView *ruleList;
@property NSArray *rules;
@property NSDictionary *lastStatus;
@property NSPopUpButton *shortcutAction;
@property NSTextField *shortcutCode;
@property NSPopUpButton *powerMode;
@property NSTableView *energyTable;
@property NSArray<NSDictionary *> *energyApps;
@property NSSegmentedControl *historyRange;
@property NSSegmentedControl *historyMetric;
@property NeroChart *chart;
@property NSTimer *timer;
@property BOOL menuTracking;
@property BOOL chargeDirty;
@property BOOL thermalDirty;
@property BOOL loaded;
@property BOOL connected;
@property NSInteger selectedPanel;
@property NSDate *historyRefreshAt;
- (NSStackView *)newPanel;
- (void)show:(id)sender;
- (void)navigate:(NSButton *)sender;
- (void)dirty:(NSControl *)sender;
- (void)refresh:(id)sender;
@end

@interface NeroController(Panels)
- (void)buildDashboard;
- (void)buildCharging;
- (void)buildCalibration;
- (void)buildHistory;
- (void)buildAutomation;
- (void)buildEnergy;

@end

@interface NeroController(Actions)
- (void)showLimitPopover:(id)sender;
- (void)previewLimit:(id)sender;
- (void)presetLimit:(NSButton *)sender;
- (void)applyQuickLimit:(id)sender;
- (void)act:(id)sender;
- (void)ruleRecurrence:(id)sender;
- (void)editRule:(NSButton *)sender;
- (void)historyChanged:(id)sender;


- (void)addRule:(id)sender;
- (void)toggleRule:(NSButton *)sender;
- (void)removeRule:(NSButton *)sender;
@end

@interface NeroController(Render)
- (void)render:(NSDictionary *)json connected:(BOOL)connected;
@end

@interface NeroController(EnergyTable) <NSTableViewDataSource>
@end

extern NeroController *neroController;
NSTextField *NeroLabel(NSString *text,CGFloat size,NSFontWeight weight);
NSStackView *NeroRow(NSArray<NSView *> *views);
void NeroSection(NSStackView *stack,NSString *title,NSString *note);
void NeroFullWidth(NSView *view,NSStackView *parent);
NSButton *NeroButton(NeroController *owner,NSString *title,NSInteger action);
NSPopUpButton *NeroPopup(NSArray<NSString *> *items,NSInteger selected);
void NeroValue(NeroController *owner,NSString *key,NSString *value);
id NeroGet(NSDictionary *json,NSString *path);
NSNumber *NeroNumber(id value);
NSString *NeroMetricString(id value,NSString *unit,NSInteger decimals);
void NeroSend(NSInteger action,id value);
NSDate *NeroDate(NSString *text);
NSColor *NeroAccent(void);
void NeroPrimaryTitle(NSButton *button,NSString *title);
NSStackView *NeroCard(NSStackView *parent);

@interface NeroController(SleepPanel)
- (void)buildSleep;
@end
