package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/http"
)

func main() {
	// Probing mode: the container's HEALTHCHECK re-executes this binary to
	// check on the daemon already running beside it. It never returns.
	healthcheck := flag.Bool("healthcheck", false,
		"probe a running daemon and exit 0 if healthy, 1 if not")
	flag.Parse()

	if *healthcheck {
		runHealthcheck()
	}

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Instantiate a new type to represent our application.
	// This type lets us shared setup code with our end-to-end tests.
	m := NewMain()
	//TODO: Load config from env

	// Execute program.
	if err := m.Run(ctx); err != nil {
		m.Close()
		fmt.Fprintln(os.Stderr, err)
		// TODO: ReportError(ctx, err)
		os.Exit(1)
	}

	// Wait for SIGINT, SIGKILL.
	<-ctx.Done()

	// Clean up program.
	if err := m.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type Main struct {
	// HTTP server for handling HTTP communication.
	HTTPServer *http.Server

	// Application services are attached to it before running.
	App app.App
}

// NewMain returns a new instance of Main.
func NewMain() *Main {
	return &Main{
		HTTPServer: http.NewServer(),
	}
}

// Close gracefully stops the program.
func (m *Main) Close() error {
	if m.HTTPServer != nil {
		if err := m.HTTPServer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Run executes the program. The configuration should already be set up before
// calling this function.
func (m *Main) Run(ctx context.Context) (err error) {
	m.App = app.App{
		UserService: &services.UserService{
			Users: inmem.NewUserRepository(),
		},
	}

	m.HTTPServer.SetApp(m.App)

	// Start the HTTP server.
	if err := m.HTTPServer.Open(); err != nil {
		return err
	}

	return nil
}
