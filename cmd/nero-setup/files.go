// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"errors"
	"fmt"
	"github.com/charlie0129/batt/pkg/localipc"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const stateDir = "/Library/Application Support/NERO Battery"
const helperPath = "/Library/PrivilegedHelperTools/io.nero.battery.helper"
const launchPath = "/Library/LaunchDaemons/io.nero.battery.helper.plist"
const policyPath = stateDir + "/ipc-policy.json"
const clientDir = stateDir + "/ControlClient"
const clientPath = clientDir + "/nero-control-client"
const label = "system/io.nero.battery.helper"

type account struct {
	UID        int
	Name, Home string
}

func accountFor(uid int) (account, error) {
	if uid < 501 {
		return account{}, errors.New("select an ordinary local user, not root or a system account")
	}
	u, e := user.LookupId(strconv.Itoa(uid))
	if e != nil {
		return account{}, e
	}
	home := filepath.Clean(u.HomeDir)
	if !strings.HasPrefix(home, "/Users/") || filepath.Dir(home) != "/Users" || home == "/Users/Shared" {
		return account{}, errors.New("this trial supports local accounts with a direct /Users home directory")
	}
	st, e := os.Lstat(home)
	if e != nil {
		return account{}, e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || int(st.Sys().(*syscall.Stat_t).Uid) != uid {
		return account{}, errors.New("unsafe user home")
	}
	return account{uid, u.Username, home}, nil
}
func (a account) apps() string { return filepath.Join(a.Home, "Applications") }
func (a account) app() string  { return filepath.Join(a.apps(), "NERO Battery.app") }
func regularUser() (account, error) {
	if os.Geteuid() == 0 {
		return account{}, errors.New("GUI operations must run as the ordinary target user")
	}
	return accountFor(os.Geteuid())
}
func (a account) validateApps() error {
	st, e := os.Lstat(a.apps())
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || int(st.Sys().(*syscall.Stat_t).Uid) != a.UID {
		return errors.New("Applications must be a real directory owned by the target user")
	}
	return nil
}
func writeJSON(p string, v any, mode fs.FileMode) error {
	b, e := jsonBytes(v)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".nero-write-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, p)
}

// os.Root confines all reads even if an untrusted source tree changes mid-copy.
func copyTree(src, dst string) error {
	r, e := os.OpenRoot(src)
	if e != nil {
		return e
	}
	defer r.Close()
	count := 0
	var total int64
	return fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if metadataFile(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected: %s", p)
		}
		dest := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			if e = os.MkdirAll(dest, 0755); e != nil {
				return e
			}
			in, e := r.Open(p)
			if e != nil {
				return e
			}
			defer in.Close()
			out, e := os.Open(dest)
			if e != nil {
				return e
			}
			defer out.Close()
			return copyQuarantine(in, out)
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("special file rejected")
		}
		count++
		if count > 2048 {
			return errors.New("copy file count bound exceeded")
		}
		in, e := r.Open(p)
		if e != nil {
			return e
		}
		defer in.Close()
		perm := fs.FileMode(0644)
		if info.Mode()&0111 != 0 {
			perm = 0755
		}
		out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if e != nil {
			return e
		}
		n, e := io.Copy(out, io.LimitReader(in, 256*1024*1024+1))
		total += n
		if e == nil {
			e = copyQuarantine(in, out)
		}
		ce := out.Close()
		if e == nil {
			e = ce
		}
		if n > 256*1024*1024 || total > 1024*1024*1024 {
			return errors.New("copy size bound exceeded")
		}
		return e
	})
}
func copyFile(src, dst string, mode fs.FileMode) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	f, e := os.CreateTemp(filepath.Dir(dst), ".nero-file-")
	if e != nil {
		return e
	}
	p := f.Name()
	defer os.Remove(p)
	if e = f.Chmod(mode); e == nil {
		var n int64
		n, e = io.Copy(f, io.LimitReader(in, 256*1024*1024+1))
		if n > 256*1024*1024 {
			e = errors.New("file copy exceeds size bound")
		}
	}
	if e == nil {
		e = copyQuarantine(in, f)
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(p, dst)
}
func asUser(a account, exe string, args ...string) ([]byte, error) {
	if os.Geteuid() == a.UID {
		return output(exe, args...)
	}
	all := []string{"-n", "-u", "#" + strconv.Itoa(a.UID), "--", exe}
	all = append(all, args...)
	return output("/usr/bin/sudo", all...)
}
func protectedExisting(p string, dir bool) error {
	_, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if dir {
		return localipc.CheckProtectedDirectory(p)
	}
	return localipc.CheckProtectedFile(p)
}
