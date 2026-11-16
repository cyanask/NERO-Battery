// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func jsonBytes(v any) ([]byte, error) {
	b, e := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), e
}
func platformCheck() error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return errors.New("this release is for Apple Silicon Macs only")
	}
	b, e := output("/usr/bin/sw_vers", "-productVersion")
	if e != nil {
		return e
	}
	major, e := strconv.Atoi(strings.Split(strings.TrimSpace(string(b)), ".")[0])
	if e != nil || major < 14 {
		return errors.New("macOS 14 or later is required; compatibility also depends on firmware preflight")
	}
	return nil
}
func stringTrim(b []byte) string { return strings.TrimSpace(string(b)) }
func run() error {
	mode := flag.String("mode", "check", "check, verify, install, uninstall, update-gui; build/internal modes are documented in source")
	release := flag.String("release", "", "unpacked release/source directory")
	uid := flag.Int("uid", os.Getuid(), "ordinary target account UID")
	backup := flag.String("backup", "", "internal GUI backup directory")
	flag.Parse()
	if *release == "" && !strings.HasPrefix(*mode, "gui-") {
		return errors.New("--release is required")
	}
	if *mode == "manifest" {
		return makeManifest(*release)
	}
	if *mode == "source-info" {
		return sourceInfo(*release)
	}
	if e := platformCheck(); e != nil {
		return e
	}
	if strings.HasPrefix(*mode, "gui-") {
		return guiOperation(*mode, *release, *backup)
	}
	root, e := filepath.Abs(*release)
	if e != nil {
		return e
	}
	if *mode == "install" || *mode == "uninstall" {
		if os.Geteuid() != 0 {
			return errors.New("first installation, protected upgrades and uninstall require system administrator authorization")
		}
		a, e := accountFor(*uid)
		if e != nil {
			return e
		}
		if e = a.validateApps(); e != nil {
			return e
		}
		if e = destinations(a); e != nil {
			return e
		}
		return maintain(*mode, root, a)
	}
	m, e := verifyRelease(root)
	if e != nil {
		return e
	}
	if *mode == "verify" {
		fmt.Println("Release checksums and exact component identities verified.")
		return nil
	}
	a, e := accountFor(*uid)
	if e != nil {
		return e
	}
	if e = a.validateApps(); e != nil {
		return e
	}
	switch *mode {
	case "check":
		if e = destinations(a); e != nil {
			return e
		}
		b, _ := jsonBytes(map[string]any{"release_verified": true, "version": m.Version, "architecture": "arm64", "target_uid": a.UID, "gui": a.app(), "root_writes": false, "hardware_control_started": false, "note": "Root-only firmware probe, installation, recovery and cross-device validation are not performed by this check."})
		fmt.Print(string(b))
		return nil
	case "update-gui":
		return updateGUI(root, m, a)
	default:
		return errors.New("unknown setup mode")
	}
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "NERO setup:", e)
		os.Exit(1)
	}
}
