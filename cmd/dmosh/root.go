package main

import (
	"github.com/spf13/cobra"

	"github.com/samwisebuze/dmost/pkg/http"
)

// newRootCommand builds the command tree. Every command is constructed against
// the *Main that owns the streams and the client, so nothing in this package
// reaches for os.Stdout or builds a client of its own.
//
// The tree is built per call rather than kept in a package-level variable:
// cobra commands carry parsed flag state, so a shared one cannot be executed
// twice, which is exactly what a test wants to do.
func (m *Main) newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dmosh",
		Short: "The Dungeon Master's Open Source Toolkit shell",
		Long: "dmosh is the terminal face of DMOST.\n\n" +
			"Run it with no arguments for the full-screen interface, or give it a\n" +
			"subcommand to get one answer and exit.",

		// A subcommand that fails has already said why; cobra printing its
		// usage under that error buries it.
		SilenceUsage: true,

		// main() reports the error, so cobra must not report it a second time.
		SilenceErrors: true,

		// Bare `dmosh` opens the TUI.
		RunE: func(cmd *cobra.Command, args []string) error {
			return m.runTUI(cmd.Context())
		},
	}

	cmd.SetIn(m.Stdin)
	cmd.SetOut(m.Stdout)
	cmd.SetErr(m.Stderr)

	cmd.PersistentFlags().StringVar(&m.Server, "server", m.Server,
		"base URL of the dmostd to talk to")

	// The client cannot be built in NewMain: --server is not parsed yet there.
	// This runs after parsing and before any RunE, including the root's.
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		m.Client = http.NewClient(http.WithServer(m.Server))
	}

	cmd.AddCommand(
		m.newHealthCommand(),
		m.newVersionCommand(),
	)

	return cmd
}
