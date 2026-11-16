// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package main

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/charlie0129/batt/pkg/daemon"
	"github.com/charlie0129/batt/pkg/version"
)

// NewDaemonCommand .
func NewDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "daemon",
		Hidden:  true,
		Short:   "Run the signed read-only observer in the foreground",
		GroupID: gAdvanced,
		RunE: func(_ *cobra.Command, _ []string) error {
			slog.Info("batt daemon starting",
				"version", version.Version,
				"commit", version.GitCommit,
			)
			return daemon.Run(configPath, unixSocketPath, false)
		},
	}

	return cmd
}
