// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charlie0129/batt/pkg/localipc"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func destinations(a account) error {
	for _, p := range []string{"/Library/PrivilegedHelperTools", "/Library/LaunchDaemons", "/Library/Application Support", stateDir, clientDir, filepath.Join(stateDir, "run"), filepath.Join(stateDir, "history")} {
		if e := protectedExisting(p, true); e != nil {
			return e
		}
	}
	for _, p := range []string{helperPath, clientPath, launchPath, policyPath, filepath.Join(stateDir, "install-record.json"), filepath.Join(stateDir, "control.json"), filepath.Join(stateDir, "preferences.json"), filepath.Join(stateDir, "batt.state.json"), filepath.Join(stateDir, "batt.sleep.json"), filepath.Join(stateDir, "run/helper.log")} {
		if e := protectedExisting(p, false); e != nil {
			return e
		}
	}
	if b, e := os.ReadFile(policyPath); e == nil {
		var c localipc.Config
		if e = json.Unmarshal(b, &c); e != nil {
			return e
		}
		if _, e = localipc.NewPolicy(c); e != nil {
			return e
		}
		if c.ServerIdentifier != "io.nero.battery.helper" || int(c.AuthorizedUID) != a.UID {
			return errors.New("existing installation belongs to a different account or policy; explicit transfer is not supported")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	return nil
}
func serviceLoaded() (bool, error) {
	c := exec.Command("/bin/launchctl", "print", label)
	b, e := c.CombinedOutput()
	if e == nil {
		return true, nil
	}
	s := strings.ToLower(string(b))
	if strings.Contains(s, "could not find service") || strings.Contains(s, "not found") {
		return false, nil
	}
	return false, fmt.Errorf("cannot verify helper service: %s", s)
}
func foreignController() error {
	// Admission only: a loaded competing service is a conflict even between
	// process restarts. Never disable or unload another product's service.
	c := exec.Command("/bin/launchctl", "print", "system/com.apphousekitchen.aldente-pro.helper")
	c.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	state, err := c.CombinedOutput()
	if err == nil || (!strings.Contains(string(state), "Could not find service") && !strings.Contains(string(state), "not found")) {
		return errors.New("AlDente helper is loaded or its state cannot be verified")
	}
	b, e := output("/bin/ps", "-axo", "comm=")
	if e != nil {
		return e
	}
	for _, line := range strings.Split(string(b), "\n") {
		p := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(p, "aldente") || strings.Contains(p, "opendente") || filepath.Base(p) == "batt" {
			return errors.New("another charge controller is running; quit/uninstall it using its own controls first")
		}
	}
	return nil
}
func stopHelper() error {
	loaded, e := serviceLoaded()
	if e != nil {
		return e
	}
	if loaded {
		if _, e = output("/bin/launchctl", "bootout", label); e != nil {
			return e
		}
	}
	for n := 0; n < 20; n++ {
		b, e := output("/bin/ps", "-axo", "comm=")
		if e != nil {
			return e
		}
		found := false
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == helperPath {
				found = true
			}
		}
		if !found {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("helper did not stop; runtime files were not replaced")
}
func startHelper() error {
	if _, e := output("/bin/launchctl", "enable", label); e != nil {
		return e
	}
	_, e := output("/bin/launchctl", "bootstrap", "system", launchPath)
	return e
}
func stopGUI(a account) {
	for _, p := range []string{a.app() + "/Contents/MacOS/nero-battery", "/Applications/NERO Battery.app/Contents/MacOS/nero-battery"} {
		_, _ = asUser(a, "/usr/bin/pkill", "-f", "^"+regexp.QuoteMeta(p)+"$")
	}
}
func health(a account) error {
	c := exec.Command("/usr/bin/sudo", "-n", "-u", "#"+strconv.Itoa(a.UID), "--", clientPath)
	c.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	c.Stdin = strings.NewReader("{\"command\":\"status\",\"payload\":\"\"}\n")
	b, e := c.Output()
	if e != nil {
		return e
	}
	var reply localipc.ControlReply
	dec := json.NewDecoder(bytes.NewReader(b))
	if e = dec.Decode(&reply); e != nil {
		return e
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	var s struct {
		Limit int    `json:"limit"`
		Lower int    `json:"lower_limit"`
		Error string `json:"last_error"`
	}
	if e = json.Unmarshal(reply.Body, &s); e != nil {
		return e
	}
	if s.Limit < 50 || s.Limit > 100 || s.Lower >= s.Limit || s.Error != "" {
		return errors.New("helper status not healthy")
	}
	return nil
}
func secureStage(release string, a account) (string, string, error) {
	if e := localipc.CheckProtectedDirectory("/Library/PrivilegedHelperTools"); e != nil {
		return "", "", e
	}
	stage, e := os.MkdirTemp("/Library/PrivilegedHelperTools", ".nero-setup-")
	if e != nil {
		return "", "", e
	}
	if e = os.Chmod(stage, 0755); e != nil {
		return "", "", e
	}
	incoming := filepath.Join(stage, "incoming")
	if e = os.Mkdir(incoming, 0700); e != nil {
		return "", "", e
	}
	if e = os.Chown(incoming, a.UID, -1); e != nil {
		return "", "", e
	}
	if _, e = asUser(a, "/usr/bin/ditto", release, incoming); e != nil {
		return "", "", fmt.Errorf("stage retained at %s: %w", stage, e)
	}
	sealed := filepath.Join(stage, "sealed")
	if e = copyTree(incoming, sealed); e != nil {
		return "", "", fmt.Errorf("stage retained at %s: %w", stage, e)
	}
	if _, e = verifyRelease(sealed); e != nil {
		return "", "", fmt.Errorf("stage retained at %s: %w", stage, e)
	}
	for _, p := range components {
		if e = inheritQuarantine(sealed, filepath.Join(sealed, p)); e != nil {
			return "", "", fmt.Errorf("quarantine preservation failed; stage retained at %s: %w", stage, e)
		}
	}
	if e = os.RemoveAll(incoming); e != nil {
		return "", "", e
	}
	return stage, sealed, nil
}

// Do not chmod or chown user-selected GUI trees as root. Only their owner's
// ordinary process handles GUI placement, while backend destinations stay fixed.
func ownBackup(p string, a account) error {
	if e := os.Mkdir(p, 0700); e != nil {
		return e
	}
	return os.Chown(p, a.UID, -1)
}
func exists(p string) bool { _, e := os.Lstat(p); return e == nil }
func requireRootOwnedLegacyApp() error {
	p := "/Applications/NERO Battery.app"
	if !exists(p) {
		return nil
	}
	st, e := os.Lstat(p)
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Sys().(*syscall.Stat_t).Uid != 0 {
		return errors.New("legacy system GUI is not root-owned; remove that copy manually first")
	}
	_, e = output("/usr/bin/codesign", "--verify", "--strict", "--deep", "-R", `=identifier "io.nero.battery.app"`, p)
	return e
}
