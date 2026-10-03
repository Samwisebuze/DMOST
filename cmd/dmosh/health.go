package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// healthTimeout bounds a probe so a wedged daemon cannot hang the shell.
const healthTimeout = 2 * time.Second

// newHealthCommand is the CLI half of the skeleton's one end-to-end slice:
// flags in, pkg/http.Client to the daemon, a line out, an exit code. The TUI
// half calls the same probeHealth below.
func (m *Main) newHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Report whether the daemon is reachable and healthy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := m.probeHealth(cmd.Context()); err != nil {
				return fmt.Errorf("%s: %w", m.Server, err)
			}
			cmd.Printf("%s: ok\n", m.Server)
			return nil
		},
	}
}

// probeHealth asks the daemon for its health, under a timeout of its own so it
// answers whether or not the caller's context has a deadline.
func (m *Main) probeHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	return m.Client.Health(ctx)
}
