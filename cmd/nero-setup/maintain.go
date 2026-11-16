// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charlie0129/batt/pkg/localipc"
)

var runtimeFiles = []string{helperPath, clientPath, launchPath, policyPath, stateDir + "/install-record.json"}

// Only these fixed, previously validated protected files participate in rollback.
type savedFile struct {
	path, backup string
	mode         os.FileMode
	present      bool
}

func saveRuntime(dir string) ([]savedFile, error) {
	if e := os.Mkdir(dir, 0700); e != nil {
		return nil, e
	}
	var saved []savedFile
	for i, p := range runtimeFiles {
		s := savedFile{path: p, backup: filepath.Join(dir, fmt.Sprint(i))}
		st, e := os.Lstat(p)
		if e == nil {
			s.present = true
			s.mode = st.Mode().Perm()
			if e = copyFile(p, s.backup, 0600); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		saved = append(saved, s)
	}
	return saved, nil
}
func restoreRuntime(saved []savedFile) error {
	var all []error
	for _, s := range saved {
		if s.present {
			all = append(all, copyFile(s.backup, s.path, s.mode))
		} else if e := os.Remove(s.path); e != nil && !os.IsNotExist(e) {
			all = append(all, e)
		}
	}
	return errors.Join(all...)
}
func protectedDir(p string) error {
	if !exists(p) {
		if e := localipc.CheckProtectedDirectory(filepath.Dir(p)); e != nil {
			return e
		}
		if e := os.Mkdir(p, 0755); e != nil {
			return e
		}
	}
	return localipc.CheckProtectedDirectory(p)
}
func maintain(mode, release string, a account) (result error) {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(interrupts)
	interrupted := func() error {
		select {
		case <-interrupts:
			return errors.New("setup interrupted; restoring the previous installation")
		default:
			return nil
		}
	}
	// flock prevents simultaneous updates; O_NOFOLLOW refuses a planted lock.
	for _, p := range []string{"/Library/PrivilegedHelperTools", "/Library/LaunchDaemons"} {
		if e := protectedDir(p); e != nil {
			return e
		}
	}
	lockPath := "/Library/PrivilegedHelperTools/.io.nero.battery.setup.lock"
	if e := protectedExisting(lockPath, false); e != nil {
		return e
	}
	fd, e := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	defer syscall.Close(fd)
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return errors.New("another NERO setup is running")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	if e = destinations(a); e != nil {
		return e
	}
	if e = foreignController(); e != nil {
		return e
	}
	if e = requireRootOwnedLegacyApp(); e != nil {
		return e
	}
	stage, sealed, e := secureStage(release, a)
	if e != nil {
		return e
	}
	defer func() {
		if result == nil {
			_ = os.RemoveAll(stage)
		} else {
			result = fmt.Errorf("%w; recovery files retained at %s", result, stage)
		}
	}()
	m, e := verifyRelease(sealed)
	if e != nil {
		return e
	}
	probe, e := output(filepath.Join(sealed, "bin/nero-battery-helper"), "--probe")
	if e != nil {
		return fmt.Errorf("firmware preflight failed without taking control: %w", e)
	}
	var caps struct {
		ReadOnly bool   `json:"read_only"`
		Mode     string `json:"charge_control_mode"`
		Adapter  bool   `json:"adapter_control"`
	}
	if e = json.Unmarshal(probe, &caps); e != nil {
		return e
	}
	if !caps.ReadOnly || caps.Mode != "legacy" || !caps.Adapter {
		return errors.New("unsupported firmware; no charging control was started")
	}
	loaded, e := serviceLoaded()
	if e != nil {
		return e
	}
	if mode == "uninstall" && !exists(policyPath) {
		return errors.New("no authorized NERO installation found; nothing removed")
	}
	for _, p := range []string{stateDir, clientDir, stateDir + "/run"} {
		if e = protectedDir(p); e != nil {
			return e
		}
	}
	saved, e := saveRuntime(filepath.Join(stage, "rollback"))
	if e != nil {
		return e
	}
	guiBackup := filepath.Join(stage, "gui-backup")
	if e = ownBackup(guiBackup, a); e != nil {
		return e
	}
	setup := filepath.Join(sealed, "bin/nero-setup")
	guiTouched := false
	stopped := false
	committed := false
	// Roll back every replacement on an ordinary failure. Recovery snapshots also
	// survive interruption; setup never deletes user settings or history.
	defer func() {
		if committed || !stopped {
			return
		}
		stopErr := stopHelper()
		if stopErr != nil {
			result = errors.Join(result, fmt.Errorf("rollback could not stop new service: %w", stopErr))
			return
		}
		restoreErr := restoreRuntime(saved)
		var guiErr error
		if guiTouched {
			_, guiErr = asUser(a, setup, "--mode", "gui-restore", "--backup", guiBackup)
		}
		var startErr error
		if loaded && restoreErr == nil {
			startErr = startHelper()
		}
		result = errors.Join(result, restoreErr, guiErr, startErr)
	}()
	stopGUI(a)
	if e = interrupted(); e != nil {
		return e
	}
	if e = stopHelper(); e != nil {
		return e
	}
	stopped = true
	if e = interrupted(); e != nil {
		return e
	}
	if mode == "uninstall" {
		// Always use the sealed *new* helper: older releases do not recognize this
		// flag. Refuse removal unless both power switches were restored/read back.
		if _, e = output(filepath.Join(sealed, "bin/nero-battery-helper"), "--restore-power"); e != nil {
			return e
		}
		if _, e = asUser(a, setup, "--mode", "gui-remove", "--backup", guiBackup); e != nil {
			return e
		}
		guiTouched = true
		for _, p := range runtimeFiles {
			if e = os.Remove(p); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
		// An absent LaunchDaemon cannot start again on reboot.
	} else {
		if e = copyFile(filepath.Join(sealed, "bin/nero-battery-helper"), helperPath, 0755); e != nil {
			return e
		}
		if e = copyFile(filepath.Join(sealed, "bin/nero-control-client"), clientPath, 0755); e != nil {
			return e
		}
		if e = copyFile(filepath.Join(sealed, "launchd/io.nero.battery.helper.plist"), launchPath, 0644); e != nil {
			return e
		}
		policy := localipc.Config{SchemaVersion: 2, AuthorizedUID: uint32(a.UID), ServerIdentifier: identifiers["helper"], ClientIdentifiers: []string{identifiers["client"]}, ServerCDHash: m.CodeHashes["helper"], ClientCDHash: m.CodeHashes["client"]}
		if _, e = localipc.NewPolicy(policy); e != nil {
			return e
		}
		if e = writeJSON(policyPath, policy, 0644); e != nil {
			return e
		}
		if _, e = asUser(a, setup, "--mode", "gui-copy", "--release", sealed, "--backup", guiBackup); e != nil {
			return e
		}
		guiTouched = true
		if e = interrupted(); e != nil {
			return e
		}
		if e = startHelper(); e != nil {
			return e
		}
		for n := 0; n < 5; n++ {
			if e = interrupted(); e != nil {
				return e
			}
			e = health(a)
			if e == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if e != nil {
			return fmt.Errorf("installed control client did not pass XPC status check: %w", e)
		}
		if e = writeJSON(stateDir+"/install-record.json", map[string]any{"schema": 1, "version": m.Version, "authorized_uid": a.UID, "home": a.Home, "control_api": 1, "source_snapshot_sha256": m.SourceSnapshot}, 0600); e != nil {
			return e
		}
	}
	// The legacy path was validated root-owned and belongs to this App. Delete
	// it only after the replacement has passed all mandatory checks.
	if e = os.RemoveAll("/Applications/NERO Battery.app"); e != nil {
		return e
	}
	committed = true
	if mode == "uninstall" {
		_ = os.Remove(clientDir)
	}
	if mode == "install" {
		_, _ = asUser(a, "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister", "-f", a.app())
	}
	fmt.Printf("%s complete for UID %d. Local preferences/history retained.\n", mode, a.UID)
	return nil
}
