// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// batt-observe is a read-only CLI, not an application bundle or an installer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charlie0129/batt/pkg/powerinfo"
)

func run() error {
	jsonOutput := flag.Bool("json", false, "write the whitelisted local observation as JSON")
	count := flag.Int("count", 1, "number of local snapshots (1..3600)")
	interval := flag.Duration("interval", time.Second, "interval between snapshots (at least 1s)")
	flag.Parse()
	if flag.NArg() != 0 || *count < 1 || *count > 3600 || *interval < time.Second {
		return fmt.Errorf("use no positional arguments, count 1..3600 and interval >= 1s")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	encoder := json.NewEncoder(os.Stdout)
	for i := 0; i < *count; i++ {
		if ctx.Err() != nil {
			return nil
		}
		snapshot, err := powerinfo.ReadObservation()
		if err != nil {
			return err
		}
		if *jsonOutput {
			if err := encoder.Encode(snapshot); err != nil {
				return err
			}
		} else if err := printStatus(snapshot); err != nil {
			return err
		}
		if i+1 < *count {
			timer := time.NewTimer(*interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
	}
	return nil
}

func printStatus(s powerinfo.Observation) error {
	b := s.Battery
	number := func(v *int64, unit string) string {
		if v == nil {
			return "未知"
		}
		return fmt.Sprintf("%d %s", *v, unit)
	}
	source := "未知"
	if b.ExternalConnected != nil {
		if *b.ExternalConnected {
			source = "外接电源"
		} else {
			source = "电池供电"
		}
	}
	state := "未知"
	if b.Charging != nil {
		if *b.Charging {
			state = "正在充电"
		} else {
			state = "未充电"
		}
	}
	if b.FullyCharged != nil && *b.FullyCharged {
		state = "已充满"
	}
	power := "未知"
	if s.Power.BatteryWatts != nil {
		power = fmt.Sprintf("%.2f W（负值表示放电）", *s.Power.BatteryWatts)
	}
	health := "未知"
	if b.RawCapacityHealth != nil {
		health = fmt.Sprintf("%.1f%%", *b.RawCapacityHealth)
	}
	_, err := fmt.Fprintf(os.Stdout,
		"NERO Battery · 只读状态\n仅在本机读取和显示，不更改充电设置。\n\n电量：%s\n电源：%s\n状态：%s\n电池功率：%s\n最大容量：%s\n设计容量：%s\n容量比例：%s\n循环次数：%s\n电池电流：%s\n\n",
		number(b.ReportedChargePercent, "%"), source, state, power,
		number(b.RawMaxCapacityMAh, "mAh"), number(b.DesignCapacityMAh, "mAh"), health,
		number(b.CycleCount, "次"), number(b.CurrentMA, "mA"))
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "batt-observe:", err)
		os.Exit(1)
	}
}
