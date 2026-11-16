// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charlie0129/batt/pkg/localipc"
)

func updateGUI(root string, m manifest, a account) error {
	if os.Geteuid() != a.UID || os.Geteuid() == 0 {
		return errors.New("GUI updates run only as the authorized ordinary account")
	}
	if e := destinations(a); e != nil {
		return e
	}
	if _, e := localipc.LoadPolicy(policyPath); e != nil {
		return e
	}
	b, e := os.ReadFile(policyPath)
	if e != nil {
		return e
	}
	var p localipc.Config
	if e = json.Unmarshal(b, &p); e != nil {
		return e
	}
	if p.SchemaVersion != 2 || p.ServerCDHash != m.CodeHashes["helper"] || p.ClientCDHash != m.CodeHashes["client"] || len(p.ClientIdentifiers) != 1 || p.ClientIdentifiers[0] != identifiers["client"] {
		return errors.New("protected components differ; use Install.command once to authorize this backend upgrade")
	}
	if e = a.validateApps(); e != nil {
		return e
	}
	if e = os.MkdirAll(a.apps(), 0700); e != nil {
		return e
	}
	backup, e := os.MkdirTemp(a.apps(), ".nero-update-backup-")
	if e != nil {
		return e
	}
	stopGUI(a)
	if e = guiOperation("gui-copy", root, backup); e != nil {
		return fmt.Errorf("GUI update failed; backup at %s: %w", backup, e)
	}
	_, e = output(filepath.Join(a.app(), "Contents/MacOS/nero-battery"), "--status")
	if e != nil {
		restore := guiOperation("gui-restore", "", backup)
		return errors.Join(e, restore)
	}
	if e = os.RemoveAll(backup); e != nil {
		return e
	}
	fmt.Println("GUI updated. Protected client, helper, policy and charge settings unchanged.")
	return nil
}
