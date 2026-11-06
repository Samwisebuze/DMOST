package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/samwisebuze/dmost/pkg/session"
)

func main() {
	// Probing mode: the container's HEALTHCHECK re-executes this binary to
	// check on the daemon already running beside it. It never returns.
	healthcheck := flag.Bool("healthcheck", false,
		"probe a running daemon and exit 0 if healthy, 1 if not")
	local := flag.Bool("local", false, "host an ephemeral local table")
	listen := flag.String("listen", ":8080", "listen address for local mode")
	advertise := flag.String("advertise", "", "LAN IPv4 address to show players (auto-detect if empty)")
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
	m.LocalMode = *local
	m.HTTPServer.Addr = *listen
	m.AdvertiseIP = *advertise
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
	App          app.App
	LocalMode    bool
	AdvertiseIP  string
	LocalSession *session.Session
	HostToken    string
}

// NewMain returns a new instance of Main.
func NewMain() *Main {
	return &Main{
		HTTPServer: http.NewServer(),
	}
}

// Close gracefully stops the program.
func (m *Main) Close() error {
	if m.LocalSession != nil {
		m.LocalSession.End()
	}
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
		CharacterService: &services.CharacterService{
			Characters: inmem.NewCharacterRepository(),
		},
	}

	m.HTTPServer.SetApp(m.App)
	if m.LocalMode {
		var err error
		m.LocalSession, err = session.New()
		if err != nil {
			return err
		}
		m.HostToken, err = randomHostToken()
		if err != nil {
			return err
		}
		m.HTTPServer.SetLocalSession(m.LocalSession, m.HostToken)
	}

	// Start the HTTP server.
	if err := m.HTTPServer.Open(); err != nil {
		return err
	}
	if m.LocalMode {
		ip := m.AdvertiseIP
		if ip == "" {
			var err error
			ip, err = localIPv4()
			if err != nil {
				return err
			}
		}
		fmt.Printf("Local table: http://%s:%d/\nJoin code: %s\nHost token: %s\n", ip, m.HTTPServer.Port(), m.LocalSession.Code(), m.HostToken)
	}

	return nil
}

func localIPv4() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no private LAN IPv4 address found; pass --advertise")
}

func randomHostToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
