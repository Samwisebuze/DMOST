// Command dmosh is the DMOST shell: one binary that is both a scriptable CLI
// and a full-screen TUI over a running dmostd.
//
// This is a walking skeleton. Every layer it will eventually need is present
// and wired — argument parsing (cobra), the terminal program (bubbletea), and
// the daemon client (pkg/http.Client) — but the only thing it actually does is
// ask the daemon whether it is healthy. Both faces run that same slice: `dmosh
// health` prints the answer and exits, `dmosh` renders it and waits. Feature
// work adds subcommands and panes to a path that already runs end to end.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/samwisebuze/dmost/pkg/http"
)

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Instantiate a new type to represent our program. This type lets us share
	// setup code with our end-to-end tests, the way cmd/dmostd's Main does.
	m := NewMain()

	if err := m.Run(ctx, os.Args[1:]); err != nil {
		m.Close()
		// cobra has already reported usage errors against Stderr; anything
		// still coming back here is the command itself failing.
		fmt.Fprintln(m.Stderr, "error:", err)
		os.Exit(1)
	}

	if err := m.Close(); err != nil {
		fmt.Fprintln(m.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Main represents the program. Its streams are fields rather than direct
// references to the os package so a test can drive the whole binary — CLI
// output and the TUI alike — through buffers.
type Main struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Server is the base URL of the dmostd this process talks to. It is bound
	// to the --server flag and read again after parsing, so a test may set it
	// directly and a user may override it.
	Server string

	// Client talks to that daemon. It is nil until Run parses the flags: the
	// address it needs is one of them.
	Client *http.Client
}

// defaultServer mirrors the listen address hard-coded in http.NewServer(); when
// that becomes configurable this should follow it.
const defaultServer = "http://127.0.0.1:8080"

// NewMain returns a new instance of Main. Like the daemon's, it allocates and
// defaults but does not connect, so it cannot fail.
func NewMain() *Main {
	return &Main{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Server: defaultServer,
	}
}

// Run parses args and executes the command they name. With no arguments that is
// the TUI, which holds the terminal until the user quits.
func (m *Main) Run(ctx context.Context, args []string) error {
	cmd := m.newRootCommand()
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}

// Close gracefully stops the program. Nothing here owns a connection yet — the
// client is stateless and the terminal is restored by bubbletea before Run
// returns — but the shape is the one the daemon uses, and a future cache or
// local store hangs off it.
func (m *Main) Close() error {
	return nil
}
