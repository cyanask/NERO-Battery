// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

const quarantineAttribute = "com.apple.quarantine"

// Copy only the macOS quarantine marker, never user-supplied ACLs or ownership.
// File descriptors keep this inside the already-open source/destination trees.
func copyQuarantine(src, dst *os.File) error {
	n, e := unix.Fgetxattr(int(src.Fd()), quarantineAttribute, nil)
	if errors.Is(e, unix.ENOATTR) {
		return nil
	}
	if e != nil {
		return e
	}
	if n > 64*1024 {
		return errors.New("quarantine attribute exceeds its size bound")
	}
	b := make([]byte, n)
	n, e = unix.Fgetxattr(int(src.Fd()), quarantineAttribute, b)
	if e != nil {
		return e
	}
	return unix.Fsetxattr(int(dst.Fd()), quarantineAttribute, b[:n], 0)
}

// Archive Utility may mark the enclosing release directory rather than every
// executable. Propagate that marker without clearing a component's own marker.
func inheritQuarantine(parent, child string) error {
	dst, e := os.Open(child)
	if e != nil {
		return e
	}
	defer dst.Close()
	_, e = unix.Fgetxattr(int(dst.Fd()), quarantineAttribute, nil)
	if e == nil {
		return nil
	}
	if !errors.Is(e, unix.ENOATTR) {
		return e
	}
	src, e := os.Open(parent)
	if e != nil {
		return e
	}
	defer src.Close()
	return copyQuarantine(src, dst)
}
