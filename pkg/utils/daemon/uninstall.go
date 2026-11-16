// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package daemon

// Uninstall must not reset SMC or remove a controller in the read-only stage.
func Uninstall() error { return ErrSignedInstallerRequired }
