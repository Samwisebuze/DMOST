package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/http/problem"
	"golang.org/x/crypto/acme/autocert"
)

// ShutdownTimeout is the time given for outstanding requests to finish before shutdown.
const ShutdownTimeout = 1 * time.Second

type Server struct {
	ln     net.Listener
	router *mux.Router
	server *http.Server
	app    *app.App

	Addr   string
	Domain string
}

func NewServer() *Server {
	s := &Server{
		router: mux.NewRouter(),
		server: &http.Server{},
		app:    &app.App{},
		Addr:   ":8080",
	}

	s.router.Use(reportPanic)

	// Our router is wrapped by another function handler to perform some
	// middleware-like tasks that cannot be performed by actual middleware.
	// This includes changing route paths for JSON endpoints & overriding methods.
	s.server.Handler = http.HandlerFunc(s.serveHTTP)

	// Setup error handling routes.
	s.router.NotFoundHandler = http.HandlerFunc(s.handleNotFound)

	s.registerUserRoutes(s.router)

	endpoints := collectLeafPaths(s.router)
	slog.Debug("http: routes registered", "routes", endpoints)
	return s
}

func (s *Server) SetApp(a app.App) { *s.app = a }

// Open validates the server options and begins listening on the bind address.
func (s *Server) Open() (err error) {
	// Open a listener on our bind address.
	if s.Domain != "" {
		s.ln = autocert.NewListener(s.Domain)
	} else {
		if s.ln, err = net.Listen("tcp", s.Addr); err != nil {
			return err
		}
	}

	// Begin serving requests on the listener. We use Serve() instead of
	// ListenAndServe() because it allows us to check for listen errors (such
	// as trying to use an already open port) synchronously.
	go s.server.Serve(s.ln)

	slog.Debug(fmt.Sprintf("http server listening on %s", s.URL()))
	return nil
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	slog.Debug("http server shutting down")

	return s.server.Shutdown(ctx)
}

// UseTLS returns true if the cert & key file are specified.
func (s *Server) UseTLS() bool {
	return s.Domain != ""
}

// Scheme returns the URL scheme for the server.
func (s *Server) Scheme() string {
	if s.UseTLS() {
		return "https"
	}
	return "http"
}

// Port returns the TCP port for the running server.
// This is useful in tests where we allocate a random port by using ":0".
func (s *Server) Port() int {
	if s.ln == nil {
		return 0
	}
	return s.ln.Addr().(*net.TCPAddr).Port
}

// URL returns the local base URL of the running server.
func (s *Server) URL() string {
	scheme, port := s.Scheme(), s.Port()

	// Use localhost unless a domain is specified.
	domain := "localhost"
	if s.Domain != "" {
		domain = s.Domain
	}

	// Return without port if using standard ports.
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return fmt.Sprintf("%s://%s", s.Scheme(), domain)
	}
	return fmt.Sprintf("%s://%s:%d", s.Scheme(), domain, s.Port())
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("request received",
		"url", r.URL.String(),
		"method", r.Method,
	)

	// Override method for forms passing "_method" value.
	if r.Method == http.MethodPost {
		switch v := r.PostFormValue("_method"); v {
		case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
			r.Method = v
		}
	}

	// Override content-type for certain extensions.
	// This allows us to easily cURL API endpoints with a ".json" or ".csv"
	// extension instead of having to explicitly set Content-type & Accept headers.
	// The extensions are removed so they don't appear in the routes.
	switch ext := path.Ext(r.URL.Path); ext {
	case ".json":
		r.Header.Set("Accept", "application/json")
		r.Header.Set("Content-type", "application/json")
		r.URL.Path = strings.TrimSuffix(r.URL.Path, ext)
	}

	// Delegate remaining HTTP handling to the gorilla router.
	s.router.ServeHTTP(w, r)
}

// handleNotFound handles requests to routes that don't exist.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	problem.New().Of(http.StatusNotFound).WriteTo(w)
}

// reportPanic is middleware for catching panics and reporting them.
func reportPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				//TODO ReportPanic(err)
				slog.Error("panic occurred", "panic", err)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func collectLeafPaths(r *mux.Router) []string {
	var leaves []string

	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		if route.GetHandler() == nil {
			return nil
		}

		t, _ := route.GetPathTemplate()
		sM, _ := route.GetMethods()
		leaves = append(leaves, fmt.Sprintf("%s %v", t, sM))
		return nil
	})

	return leaves
}
