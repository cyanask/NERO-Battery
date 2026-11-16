// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/charlie0129/batt/pkg/compatibility"
	daemonutils "github.com/charlie0129/batt/pkg/utils/daemon"
)

func TestPhaseOneInstallDoesNotProvisionAnything(t *testing.T) {
	oldPath := configPath
	configPath = filepath.Join(t.TempDir(), "must-not-be-created.json")
	t.Cleanup(func() { configPath = oldPath })
	for _, cmd := range []*cobra.Command{NewInstallCommand(), NewUninstallCommand()} {
		if err := cmd.RunE(cmd, nil); !errors.Is(err, daemonutils.ErrSignedInstallerRequired) {
			t.Fatalf("%s must require the future signed installer, got %v", cmd.Name(), err)
		}
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("disabled installer touched configuration: %v", err)
	}
	if NewCommand().PersistentFlags().Lookup("pprof") != nil {
		t.Fatal("read-only first stage must not expose a TCP profiler flag")
	}
	if NewDaemonCommand().Flags().Lookup("always-allow-non-root-access") != nil {
		t.Fatal("legacy broad-access daemon flag must not remain available")
	}
}

func TestRequiredCapabilityInheritedFromParentCommand(t *testing.T) {
	root := &cobra.Command{Use: "batt"}
	parent := annotateCapability(&cobra.Command{Use: "adapter"}, compatibility.FeatureAdapterControl)
	child := &cobra.Command{Use: "disable"}
	parent.AddCommand(child)
	root.AddCommand(parent)

	got, ok := requiredCapability(child)
	if !ok || got != compatibility.FeatureAdapterControl {
		t.Fatalf("requiredCapability() = %q, %t", got, ok)
	}
}

func TestHideUnsupportedCommands(t *testing.T) {
	root := NewCommand()
	hideUnsupportedCommands(root, compatibility.Capabilities{
		ChargingControl:   true,
		ChargeControlMode: compatibility.ChargeControlFirmware,
	})

	tests := map[string]bool{
		"limit":                            false,
		"lower-limit-delta":                false,
		"status":                           false,
		"adapter":                          true,
		"prevent-idle-sleep":               true,
		"disable-charging-pre-sleep":       true,
		"prevent-system-sleep":             true,
		"prevent-sleep-on-adapter-disable": true,
		"magsafe-led":                      true,
		"calibration":                      true,
		"schedule":                         true,
	}
	for name, wantHidden := range tests {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if cmd.Hidden != wantHidden {
			t.Errorf("%s hidden = %t, want %t", name, cmd.Hidden, wantHidden)
		}
	}
}
