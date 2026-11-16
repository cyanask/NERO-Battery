// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package localipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type authenticatedListener struct {
	*net.UnixListener
	policy *Policy
}

// Listen never creates directories or replaces an existing endpoint. Provisioning
// the protected namespace belongs to the future signed installer.
func Listen(path string, policy *Policy) (net.Listener, error) {
	if policy == nil {
		return nil, errors.New("IPC signing policy is required")
	}
	if err := checkServerExecutable(); err != nil {
		return nil, err
	}
	if err := policy.VerifySelf(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("IPC socket path must be absolute and clean")
	}
	if err := checkRootDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(path); err == nil {
		owner, ok := st.Sys().(*syscall.Stat_t)
		if !ok || st.Mode()&os.ModeSocket == 0 || owner.Uid != policy.uid || st.Mode().Perm() != 0o600 {
			return nil, errors.New("refusing an unexpected existing IPC endpoint")
		}
		c, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
		if dialErr == nil {
			c.Close()
			return nil, errors.New("IPC endpoint is already active")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, dialErr
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		l.Close()
		return nil, err
	}
	if err := os.Chown(path, int(policy.uid), -1); err != nil {
		l.Close()
		return nil, err
	}
	return &authenticatedListener{UnixListener: l, policy: policy}, nil
}

func (l *authenticatedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.AcceptUnix()
		if err != nil {
			return nil, err
		}
		if err := l.policy.VerifyPeer(conn); err != nil {
			conn.Close()
			continue
		}
		return conn, nil
	}
}
