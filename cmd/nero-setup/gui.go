// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// These modes always execute as the target user; root never writes through a
// user-mutable Applications path. The caller provides a private backup folder.
func guiOperation(mode, release, backup string) error {
	a, e := regularUser()
	if e != nil {
		return e
	}
	if e = a.validateApps(); e != nil {
		return e
	}
	if e = os.MkdirAll(a.apps(), 0700); e != nil {
		return e
	}
	if backup == "" {
		return errors.New("GUI backup directory required")
	}
	old := filepath.Join(backup, "old-app")
	if mode == "gui-restore" {
		_, e := os.Stat(filepath.Join(backup, "had-app"))
		if e == nil {
			tmp, e := os.MkdirTemp(a.apps(), ".nero-restore-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(tmp)
			if e = copyTree(old, filepath.Join(tmp, "app")); e != nil {
				return e
			}
			if e = os.RemoveAll(a.app()); e != nil {
				return e
			}
			return os.Rename(filepath.Join(tmp, "app"), a.app())
		}
		if os.IsNotExist(e) {
			return os.RemoveAll(a.app())
		}
		return e
	}
	if mode != "gui-copy" && mode != "gui-remove" {
		return errors.New("unknown GUI operation")
	}
	if _, e = os.Lstat(a.app()); e == nil {
		st, e := os.Lstat(a.app())
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || int(st.Sys().(*syscall.Stat_t).Uid) != a.UID {
			return errors.New("unexpected existing GUI destination")
		}
		if e = copyTree(a.app(), old); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(backup, "had-app"), []byte("1\n"), 0600); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	tmp, e := os.MkdirTemp(a.apps(), ".nero-gui-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	if mode == "gui-copy" {
		if e = copyTree(filepath.Join(release, "app/NERO Battery.app"), filepath.Join(tmp, "app")); e != nil {
			return e
		}
		if e = inheritQuarantine(release, filepath.Join(tmp, "app")); e != nil {
			return e
		}
		if _, e = output("/usr/bin/codesign", "--verify", "--strict", "--deep", filepath.Join(tmp, "app")); e != nil {
			return e
		}
	}
	moved := false
	if _, e = os.Lstat(a.app()); e == nil {
		if e = os.Rename(a.app(), filepath.Join(tmp, "old")); e != nil {
			return e
		}
		moved = true
	}
	if mode == "gui-copy" {
		if e = os.Rename(filepath.Join(tmp, "app"), a.app()); e != nil {
			if moved {
				_ = os.Rename(filepath.Join(tmp, "old"), a.app())
			}
			return e
		}
	}
	return nil
}
