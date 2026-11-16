// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#import "native.h"
@implementation NeroController(EnergyTable)
- (NSInteger)numberOfRowsInTableView:(NSTableView *)table {return self.energyApps.count;}
- (id)tableView:(NSTableView *)table objectValueForTableColumn:(NSTableColumn *)column row:(NSInteger)row {
    if(row<0||row>=self.energyApps.count)return @"";
    NSDictionary *app=self.energyApps[row];
    if([column.identifier isEqualToString:@"name"])return app[@"name"]?:@"—";
    if([column.identifier isEqualToString:@"cpu"])return NeroMetricString(app[@"cpu_percent"],@"%",1);
    return NeroMetricString(app[@"energy_impact"],@"",1);
}
@end
