// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// Package localipc authenticates local callers without opening Internet sockets.
package localipc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

const (
	DefaultPolicyPath = "/Library/Application Support/NERO Battery/ipc-policy.json"
	DefaultSocketPath = "/Library/Application Support/NERO Battery/run/observer.sock"
	maxPolicyBytes    = 16 * 1024
)

var (
	teamPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,254}$`)
)

// Config contains public signing metadata, never a credential or private key.
type Config struct {
	SchemaVersion     int      `json:"schema_version"`
	AuthorizedUID     uint32   `json:"authorized_uid"`
	TeamID            string   `json:"team_id"`
	ServerIdentifier  string   `json:"server_identifier"`
	ClientIdentifiers []string `json:"client_identifiers"`
	ServerCDHash      string   `json:"server_cdhash,omitempty"`
	ClientCDHash      string   `json:"client_cdhash,omitempty"`
}

// Policy is immutable after validation; an empty Policy cannot authorize peers.
type Policy struct {
	uid               uint32
	serverRequirement string
	clientRequirement string
}

func NewPolicy(c Config) (*Policy, error) {
	// A local release needs no certificate authority. Its exact sealed code
	// hashes are provisioned by the administrator, never learned from a peer.
	if c.SchemaVersion == 2 {
		hashPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)
		if c.AuthorizedUID == 0 || c.TeamID != "" || !idPattern.MatchString(c.ServerIdentifier) || len(c.ClientIdentifiers) != 1 || !idPattern.MatchString(c.ClientIdentifiers[0]) || !hashPattern.MatchString(c.ServerCDHash) || !hashPattern.MatchString(c.ClientCDHash) || c.ServerCDHash == c.ClientCDHash {
			return nil, errors.New("local release policy requires a non-root account, distinct exact code hashes and one client")
		}
		return &Policy{uid: c.AuthorizedUID,
			serverRequirement: `identifier "` + c.ServerIdentifier + `" and cdhash H"` + c.ServerCDHash + `"`,
			clientRequirement: `identifier "` + c.ClientIdentifiers[0] + `" and cdhash H"` + c.ClientCDHash + `"`}, nil
	}
	if c.SchemaVersion != 1 || c.AuthorizedUID == 0 || !teamPattern.MatchString(c.TeamID) {
		return nil, errors.New("IPC policy requires schema 1, a non-root account and a real signing Team ID")
	}
	if !idPattern.MatchString(c.ServerIdentifier) || len(c.ClientIdentifiers) == 0 || len(c.ClientIdentifiers) > 8 {
		return nil, errors.New("IPC policy has an invalid server or client identifier list")
	}
	ids := slices.Clone(c.ClientIdentifiers)
	slices.Sort(ids)
	clauses := make([]string, len(ids))
	for i, id := range ids {
		if !idPattern.MatchString(id) || (i > 0 && ids[i-1] == id) {
			return nil, errors.New("IPC client identifiers must be valid and unique")
		}
		clauses[i] = `identifier "` + id + `"`
	}
	// Require Developer ID Application, not a development or ad-hoc identity.
	prefix := `anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = "` + c.TeamID + `" and `
	return &Policy{
		uid:               c.AuthorizedUID,
		serverRequirement: prefix + `identifier "` + c.ServerIdentifier + `"`,
		clientRequirement: prefix + "(" + strings.Join(clauses, " or ") + ")",
	}, nil
}

// LoadPolicy accepts only a protected, root-owned policy. It never creates it.
func LoadPolicy(path string) (*Policy, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("IPC policy path must be absolute and clean")
	}
	if err := checkRootDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open protected IPC policy: %w", err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.Mode().IsRegular() || owner.Uid != 0 || st.Mode().Perm()&0o133 != 0 {
		return nil, errors.New("public IPC policy must be a root-owned regular file with mode 0644 or stricter")
	}
	if err := checkNoExtendedACLFD(fd); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPolicyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPolicyBytes {
		return nil, errors.New("IPC policy is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var c Config
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("decode IPC policy: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("IPC policy must contain exactly one JSON object")
	}
	return NewPolicy(c)
}

func checkRootDirectory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("IPC directory must be absolute and clean")
	}
	for {
		st, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect protected IPC directory: %w", err)
		}
		owner, ok := st.Sys().(*syscall.Stat_t)
		if !ok || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || owner.Uid != 0 || st.Mode().Perm()&0o022 != 0 {
			return errors.New("IPC directory and ancestors must be root-owned, without symlinks or group/other write access")
		}
		if err := checkNoExtendedACL(path); err != nil {
			return err
		}
		if path == "/" {
			return nil
		}
		path = filepath.Dir(path)
	}
}

// CheckProtectedDirectory is shared by the installer and runtime trust checks.
func CheckProtectedDirectory(path string) error { return checkRootDirectory(path) }

// CheckProtectedFile rejects unsafe pre-existing privileged destinations.
func CheckProtectedFile(path string) error {
	if err := checkRootDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.Mode().IsRegular() || owner.Uid != 0 || st.Mode().Perm()&0022 != 0 {
		return errors.New("protected file must be root-owned and non-writable by other users")
	}
	return checkNoExtendedACL(path)
}

func checkServerExecutable() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	if err := checkRootDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.Mode().IsRegular() || owner.Uid != 0 || st.Mode().Perm()&0o022 != 0 {
		return errors.New("observer executable must be a root-owned, non-user-writable regular file")
	}
	return checkNoExtendedACL(path)
}

// XPCIdentity supplies immutable public requirements to Foundation. The framework
// enforces them against kernel-supplied peer identities for every connection.
func (p *Policy) XPCIdentity() (uint32, string, string, error) {
	if p == nil || p.uid == 0 || p.serverRequirement == "" || p.clientRequirement == "" {
		return 0, "", "", errors.New("validated XPC signing policy required")
	}
	return p.uid, p.serverRequirement, p.clientRequirement, nil
}
