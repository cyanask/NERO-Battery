// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type localAppEnergy struct {
	Name         string   `json:"name"`
	CPUPercent   *float64 `json:"cpu_percent"`
	EnergyImpact *float64 `json:"energy_impact"`
}
type localEnergyReport struct {
	At      time.Time        `json:"at"`
	Source  string           `json:"source"`
	CPUW    *float64         `json:"cpu_w"`
	GPUW    *float64         `json:"gpu_w"`
	ANEW    *float64         `json:"ane_w"`
	Apps    []localAppEnergy `json:"apps"`
	Message string           `json:"message"`
}

type boundedCommandOutput struct {
	bytes.Buffer
	max int
}

func (b *boundedCommandOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func sampleLocalEnergy() localEnergyReport {
	result := localEnergyReport{At: time.Now().UTC(), Source: "macOS powermetrics · 系统估算", Apps: []localAppEnergy{}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/powermetrics", "--samplers", "tasks,cpu_power,gpu_power,ane_power", "--show-process-energy", "--show-process-samp-norm", "-i", "1000", "-n", "1", "-f", "text")
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	output := &boundedCommandOutput{max: 8 * 1024 * 1024}
	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		result.Message = "此系统暂未提供能耗采样；未生成估算值"
		return result
	}
	return parseLocalEnergy(output.String(), result)
}

func parseLocalEnergy(text string, result localEnergyReport) localEnergyReport {
	for _, item := range []struct {
		label string
		dest  **float64
	}{{"CPU", &result.CPUW}, {"GPU", &result.GPUW}, {"ANE", &result.ANEW}} {
		re := regexp.MustCompile(`(?mi)^\s*` + item.label + ` Power:\s*([0-9.]+)\s*mW\s*$`)
		if match := re.FindStringSubmatch(text); len(match) == 2 {
			if f, err := strconv.ParseFloat(match[1], 64); err == nil && f >= 0 {
				v := f / 1000
				*item.dest = &v
			}
		}
	}
	columns := regexp.MustCompile(`\s{2,}`)
	cpuIndex, energyIndex, nameIndex, pidIndex := -1, -1, 0, -1
	inTasks := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.Contains(line, "CPU ms/s") && (strings.Contains(line, "Name") || strings.Contains(line, "Process")) {
			header := columns.Split(trim, -1)
			cpuIndex, energyIndex, pidIndex = -1, -1, -1
			for i, h := range header {
				if h == "ID" || h == "PID" {
					pidIndex = i
				}
				if strings.Contains(h, "CPU ms/s") {
					cpuIndex = i
				}
				if strings.Contains(strings.ToLower(h), "energy") {
					energyIndex = i
				}
				if h == "Name" || h == "Process" {
					nameIndex = i
				}
			}
			inTasks = cpuIndex >= 0
			continue
		}
		if !inTasks {
			continue
		}
		if strings.HasPrefix(trim, "***") {
			inTasks = false
			continue
		}
		fields := columns.Split(trim, -1)
		if len(fields) <= cpuIndex || len(fields) <= nameIndex {
			continue
		}
		if pidIndex < 0 || pidIndex >= len(fields) {
			continue
		}
		pid, err := strconv.Atoi(fields[pidIndex])
		if err != nil || pid < 0 {
			continue
		}
		ms, err := strconv.ParseFloat(strings.TrimSuffix(fields[cpuIndex], "%"), 64)
		if err != nil || ms < 0 {
			continue
		}
		name := filepath.Base(strings.TrimSpace(fields[nameIndex]))
		if name == "" || name == "Name" || name == "ALL_TASKS" || name == "TOTAL" {
			continue
		}
		percent := ms / 10
		app := localAppEnergy{Name: name, CPUPercent: &percent}
		if energyIndex >= 0 && energyIndex < len(fields) {
			if v, err := strconv.ParseFloat(fields[energyIndex], 64); err == nil && v >= 0 {
				app.EnergyImpact = &v
			}
		}
		result.Apps = append(result.Apps, app)
	}
	sort.SliceStable(result.Apps, func(i, j int) bool {
		a, b := result.Apps[i], result.Apps[j]
		if a.EnergyImpact != nil || b.EnergyImpact != nil {
			if a.EnergyImpact == nil {
				return false
			}
			if b.EnergyImpact == nil {
				return true
			}
			return *a.EnergyImpact > *b.EnergyImpact
		}
		if a.CPUPercent == nil {
			return false
		}
		if b.CPUPercent == nil {
			return true
		}
		return *a.CPUPercent > *b.CPUPercent
	})
	if len(result.Apps) > 12 {
		result.Apps = result.Apps[:12]
	}
	result.Message = "单次约 1 秒采样；应用能耗影响为系统指标，不等于瓦数。"
	if len(result.Apps) == 0 {
		result.Message = "本机未提供可识别的应用能耗字段；未生成替代分数。"
	}
	return result
}

type localPowerMode struct {
	Supported bool   `json:"supported"`
	AC        string `json:"ac"`
	Battery   string `json:"battery"`
	HighPower bool   `json:"high_power"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

func readLocalPowerMode() localPowerMode {
	r := localPowerMode{Source: "macOS pmset", AC: "unknown", Battery: "unknown", Message: "本机未提供高功率模式"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cap, err := exec.CommandContext(ctx, "/usr/bin/pmset", "-g", "cap").Output()
	if err != nil || !strings.Contains(string(cap), "lowpowermode") {
		return r
	}
	b, err := exec.CommandContext(ctx, "/usr/bin/pmset", "-g", "custom").Output()
	if err != nil {
		return r
	}
	section := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "Battery Power:") {
			section = "battery"
		}
		if strings.Contains(line, "AC Power:") {
			section = "ac"
		}
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "lowpowermode" {
			mode := "automatic"
			if f[1] == "1" {
				mode = "low-power"
			}
			if section == "battery" {
				r.Battery = mode
			}
			if section == "ac" {
				r.AC = mode
			}
		}
	}
	r.Supported = r.AC != "unknown" || r.Battery != "unknown"
	return r
}

func setLocalPowerMode(mode string) error {
	arg := "0"
	if mode == "low-power" {
		arg = "1"
	} else if mode != "automatic" {
		return errors.New("this Mac supports automatic or low-power mode only")
	}
	if !readLocalPowerMode().Supported {
		return errors.New("power mode control is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/pmset", "-a", "lowpowermode", arg)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	if err := cmd.Run(); err != nil {
		return errors.New("macOS did not accept this power mode")
	}
	return nil
}
