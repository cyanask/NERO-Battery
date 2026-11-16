// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package gui

import (
	daemonutils "github.com/charlie0129/batt/pkg/utils/daemon"
	pkgerrors "github.com/pkg/errors"
	"log/slog"
	"os"
)

func isDaemonInstalled() bool {
	_, err := os.Stat("/Library/LaunchDaemons/io.nero.battery.observer.plist")
	return err == nil
}

// A signed installer must provision the protected observer; never invoke an
// administrator shell or remove another controller from this read-only stage.
func installDaemon(_ string) error   { return daemonutils.ErrSignedInstallerRequired }
func uninstallDaemon(_ string) error { return daemonutils.ErrSignedInstallerRequired }

func startAppAtBoot() error {
	if IsLoginItemRegistered() {
		slog.Info("Application is already registered to start at login")
		return nil
	}
	if err := RegisterLoginItem(); err != nil {
		return pkgerrors.Wrapf(err, "failed to register application to start at login")
	}
	slog.Info("Application registered to start at login")
	return nil
}
