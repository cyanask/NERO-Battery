// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
//go:build !brew

package main

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	daemonutils "github.com/charlie0129/batt/pkg/utils/daemon"
)

func init() {
	// Only non-homebrew version has install and uninstall commands.
	commandGroups = append(commandGroups, gInstallation)
}

// NewInstallCommand .
func NewInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use: "install", Short: "Self-installation disabled in the read-only first stage", GroupID: gInstallation,
		RunE: func(_ *cobra.Command, _ []string) error { return daemonutils.Install() },
	}
}

type uninstallHardware interface {
	IsChargingControlCapable() bool
	ResetChargeControl() error
	IsAdapterControlCapable() bool
	EnableAdapter() error
}

// restoreUninstallState puts the Mac back into its normal state once the daemon
// is gone: wall power first, then the system sleep setting. Nothing restores the
// sleep setting after this command, so a failed charge-limit reset must not skip
// it. Every failure is reported after all steps ran. An adapter that stays cut
// is the exception: sleep stays held then, because the Mac still runs on battery.
func restoreUninstallState(hardware uninstallHardware, resetCharging bool, recoverSleep func() error) error {
	var resetErr error
	if resetCharging && hardware.IsChargingControlCapable() {
		slog.Info("resetting charge limits")
		resetErr = hardware.ResetChargeControl()
	}
	if hardware.IsAdapterControlCapable() {
		if err := hardware.EnableAdapter(); err != nil {
			return fmt.Errorf("failed to enable adapter (charge-limit reset: %v): %w", resetErr, err)
		}
	}

	var errs []error
	if resetErr != nil {
		errs = append(errs, fmt.Errorf("failed to reset charge control: %w", resetErr))
	}
	if err := recoverSleep(); err != nil {
		errs = append(errs, fmt.Errorf("failed to restore system sleep setting: %w", err))
	}
	return errors.Join(errs...)
}

// NewUninstallCommand .
func NewUninstallCommand() *cobra.Command {
	return &cobra.Command{Use: "uninstall", Short: "Privileged removal disabled in the read-only first stage", GroupID: gInstallation,
		RunE: func(_ *cobra.Command, _ []string) error { return daemonutils.Uninstall() },
	}
}
