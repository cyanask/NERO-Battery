// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

func sourceInfo(root string) error {
	if _, e := os.Lstat(filepath.Join(root, ".git")); e == nil {
		return errors.New("export a clean source snapshot before building a release")
	}
	h := sha256.New()
	count := 0
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source symlink rejected: %s", rel)
		}
		if d.IsDir() || metadataFile(p) || rel == "SOURCE-INFO.json" {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("source contains a special file")
		}
		digest, e := digestFile(p)
		if e != nil {
			return e
		}
		fmt.Fprintf(h, "%s\x00%s\n", digest, filepath.ToSlash(rel))
		count++
		return nil
	})
	if e != nil {
		return e
	}
	v, e := os.ReadFile(filepath.Join(root, "VERSION"))
	if e != nil {
		return e
	}
	return writeJSON(filepath.Join(root, "SOURCE-INFO.json"), map[string]any{"schema": 1, "version": stringTrim(v), "snapshot_sha256": hex.EncodeToString(h.Sum(nil)), "file_count": count, "go_toolchain": runtime.Version(), "digest_format": "SHA256 of sorted (file SHA256 hex + NUL + relative path + LF), excluding SOURCE-INFO.json and Finder metadata"}, 0644)
}
