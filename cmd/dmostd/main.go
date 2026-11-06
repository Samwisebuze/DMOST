package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	localHTTPS := flag.Bool("local-https", false, "serve a local table over HTTPS")
	listen := flag.String("listen", ":8080", "listen address for local mode")
	bootstrapListen := flag.String("bootstrap-listen", ":8081", "HTTP bootstrap listen address")
	certDir := flag.String("cert-dir", ".dmost-local", "directory for the local CA")
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
	m.LocalMode = *local || *localHTTPS
	m.LocalHTTPS = *localHTTPS
	m.HTTPServer.Addr = *listen
	m.AdvertiseIP = *advertise
	m.BootstrapAddr = *bootstrapListen
	m.CertDir = *certDir
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
	App             app.App
	LocalMode       bool
	LocalHTTPS      bool
	AdvertiseIP     string
	BootstrapAddr   string
	CertDir         string
	BootstrapServer *stdhttp.Server
	localCAPEM      []byte
	LocalSession    *session.Session
	HostToken       string
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
	if m.BootstrapServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.BootstrapServer.Shutdown(ctx); err != nil {
			return err
		}
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
	var ip string
	if m.LocalMode {
		ip = m.AdvertiseIP
		if ip == "" {
			var err error
			ip, err = localIPv4()
			if err != nil {
				return err
			}
		}
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid advertised IP %q", ip)
		}
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
		if m.LocalHTTPS {
			cert, caPEM, err := localCertificate(m.CertDir, ip)
			if err != nil {
				return err
			}
			m.HTTPServer.LocalTLS = &cert
			m.localCAPEM = caPEM
		}
	}

	// Start the HTTP server.
	if err := m.HTTPServer.Open(); err != nil {
		return err
	}
	if m.LocalMode {
		if m.LocalHTTPS {
			port, err := m.startBootstrap(ip)
			if err != nil {
				_ = m.HTTPServer.Close()
				return err
			}
			fmt.Printf("Open on your device: http://%s:%d/\n", ip, port)
			block, _ := pem.Decode(m.localCAPEM)
			sum := sha256.Sum256(block.Bytes)
			fmt.Printf("CA fingerprint: %s\n", hex.EncodeToString(sum[:]))
		} else {
			fmt.Printf("Local table: http://%s:%d/\n", ip, m.HTTPServer.Port())
		}
		fmt.Printf("Join code: %s\nHost token: %s\n", m.LocalSession.Code(), m.HostToken)
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
