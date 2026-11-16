// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

import "errors"

// ErrSignedInstallerRequired prevents the legacy self-install path from turning
// a mutable, user-selected executable into a root launchd service.
var ErrSignedInstallerRequired = errors.New("self-installation is disabled: a reviewed, signed installer and protected helper location are required")

// Install is deliberately unavailable in the read-only first stage. It performs
// no chmod, config write, service removal or administrator command.
func Install() error { return ErrSignedInstallerRequired }
