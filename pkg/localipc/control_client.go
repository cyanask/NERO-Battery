// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package localipc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const ControlClientPath = "/Library/Application Support/NERO Battery/ControlClient/nero-control-client"

// VerifyControlClient binds the fixed broker to the existing protected policy.
// The editable GUI is not a trusted XPC peer and cannot supply a broker path.
func (p *Policy) VerifyControlClient() error {
	if p == nil || os.Geteuid() != int(p.uid) {
		return errors.New("control client requires the authorized user account")
	}
	if err := checkRootDirectory(filepath.Dir(ControlClientPath)); err != nil {
		return err
	}
	st, err := os.Lstat(ControlClientPath)
	if err != nil {
		return err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.Mode().IsRegular() || owner.Uid != 0 || st.Mode().Perm()&0022 != 0 {
		return errors.New("control client must be root-owned and non-user-writable")
	}
	if err := checkNoExtendedACL(ControlClientPath); err != nil {
		return err
	}
	cmd := exec.Command("/usr/bin/codesign", "--verify", "--strict", "-R", "="+p.clientRequirement, ControlClientPath)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	if err := cmd.Run(); err != nil {
		return errors.New("protected control client signature does not match policy")
	}
	return nil
}

// The protocol is local stdio only; commands are still checked by localxpc.
type ControlRequest struct {
	Command string `json:"command"`
	Payload string `json:"payload"`
}
type ControlReply struct {
	Body  []byte `json:"body,omitempty"`
	Error string `json:"error,omitempty"`
}
