// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// NERO portable release tooling. Distributed under the repository's GPLv2 license.
package main

import (
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const manifestName = "release-manifest.json"

var components = map[string]string{"gui": "app/NERO Battery.app/Contents/MacOS/nero-battery", "helper": "bin/nero-battery-helper", "client": "bin/nero-control-client", "setup": "bin/nero-setup"}
var identifiers = map[string]string{"gui": "io.nero.battery.app", "helper": "io.nero.battery.helper", "client": "io.nero.battery.controlclient", "setup": "io.nero.battery.setup"}

type manifest struct {
	Schema         int               `json:"schema"`
	Version        string            `json:"version"`
	Architecture   string            `json:"architecture"`
	MinimumMacOS   int               `json:"minimum_macos_major"`
	SourceSnapshot string            `json:"source_snapshot_sha256"`
	Files          map[string]string `json:"files"`
	CodeHashes     map[string]string `json:"code_hashes"`
}

func metadataFile(p string) bool {
	n := filepath.Base(p)
	return n == ".DS_Store" || strings.HasPrefix(n, "._")
}
func output(name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	b, e := c.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(name), e, strings.TrimSpace(string(b)))
	}
	return b, nil
}
func codeHash(p string) (string, error) {
	b, e := output("/usr/bin/codesign", "-dvvv", p)
	if e != nil {
		return "", e
	}
	m := regexp.MustCompile(`(?m)^CDHash=([a-f0-9]{40})$`).FindSubmatch(b)
	if len(m) != 2 {
		return "", errors.New("missing exact code hash")
	}
	return string(m[1]), nil
}
func verifyCode(p, id, h string) error {
	_, e := output("/usr/bin/codesign", "--verify", "--strict", "-R", `=identifier "`+id+`" and cdhash H"`+h+`"`, p)
	return e
}
func digestFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, 256*1024*1024+1))
	if e != nil {
		return "", e
	}
	if n > 256*1024*1024 {
		return "", errors.New("release file too large")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func collectFiles(root string) (map[string]string, error) {
	out := map[string]string{}
	var total int64
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in release: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		if metadataFile(p) || rel == manifestName {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a regular release file: %s", rel)
		}
		total += info.Size()
		if total > 1024*1024*1024 {
			return errors.New("release exceeds total size bound")
		}
		h, e := digestFile(p)
		if e != nil {
			return e
		}
		out[filepath.ToSlash(rel)] = h
		if len(out) > 2048 {
			return errors.New("release file count exceeds bound")
		}
		return nil
	})
	return out, e
}
func makeManifest(root string) error {
	version, e := os.ReadFile(filepath.Join(root, "VERSION"))
	if e != nil {
		return e
	}
	source, e := os.ReadFile(filepath.Join(root, "SOURCE-INFO.json"))
	if e != nil {
		return e
	}
	var sourceInfo struct {
		SHA256 string `json:"snapshot_sha256"`
	}
	if e = json.Unmarshal(source, &sourceInfo); e != nil {
		return e
	}
	m := manifest{Schema: 1, Version: strings.TrimSpace(string(version)), Architecture: "arm64", MinimumMacOS: 14, SourceSnapshot: sourceInfo.SHA256, CodeHashes: map[string]string{}}
	for role, p := range components {
		h, e := codeHash(filepath.Join(root, p))
		if e != nil {
			return e
		}
		m.CodeHashes[role] = h
	}
	m.Files, e = collectFiles(root)
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, manifestName), append(b, '\n'), 0644)
}
func verifyRelease(root string) (manifest, error) {
	var m manifest
	f, e := os.Open(filepath.Join(root, manifestName))
	if e != nil {
		return m, e
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&m); e != nil {
		return m, e
	}
	if e = decoder.Decode(new(any)); e != io.EOF {
		return m, errors.New("manifest must contain exactly one object")
	}
	if m.Schema != 1 || m.Architecture != "arm64" || m.MinimumMacOS != 14 || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(m.Version) || len(m.Files) == 0 || len(m.Files) > 2048 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(m.SourceSnapshot) {
		return m, errors.New("invalid release manifest")
	}
	files, e := collectFiles(root)
	if e != nil {
		return m, e
	}
	if len(files) != len(m.Files) {
		return m, errors.New("unlisted or missing release files")
	}
	for p, h := range m.Files {
		if files[p] != h {
			return m, fmt.Errorf("release checksum mismatch: %s", p)
		}
	}
	for role, p := range components {
		binary, e := macho.Open(filepath.Join(root, p))
		if e != nil {
			return m, e
		}
		arm := binary.Cpu == macho.CpuArm64
		_ = binary.Close()
		if !arm {
			return m, errors.New("release components must be thin ARM64 Mach-O binaries")
		}
		h := m.CodeHashes[role]
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(h) {
			return m, errors.New("invalid component identity")
		}
		if e = verifyCode(filepath.Join(root, p), identifiers[role], h); e != nil {
			return m, e
		}
	}
	version, e := output("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", filepath.Join(root, "app/NERO Battery.app/Contents/Info.plist"))
	if e != nil {
		return m, e
	}
	if strings.TrimSpace(string(version)) != m.Version {
		return m, errors.New("bundle/release version mismatch")
	}
	versionFile, e := os.ReadFile(filepath.Join(root, "VERSION"))
	if e != nil || stringTrim(versionFile) != m.Version {
		return m, errors.New("VERSION/release mismatch")
	}
	source, e := os.ReadFile(filepath.Join(root, "SOURCE-INFO.json"))
	if e != nil {
		return m, e
	}
	var info struct {
		Version string `json:"version"`
		Digest  string `json:"snapshot_sha256"`
	}
	if e = json.Unmarshal(source, &info); e != nil {
		return m, e
	}
	if info.Version != m.Version || info.Digest != m.SourceSnapshot {
		return m, errors.New("source/release mismatch")
	}
	return m, nil
}
