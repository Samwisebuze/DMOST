package main_test

import (
	"context"
	"testing"

	main "github.com/samwisebuze/dmost/cmd/dmostd"
	"github.com/samwisebuze/dmost/pkg/http"
)

// MustRunMain is a test helper function that executes Main in a temporary path.
// The HTTP server binds to ":0" so it will start on a random port. This allows
// our end-to-end tests to be run in parallel. Fail on error.
func MustRunMain(tb testing.TB) *main.Main {
	tb.Helper()

	m := main.NewMain()
	m.HTTPServer.Addr = ":0"

	if err := m.Run(context.Background()); err != nil {
		tb.Fatal(err)
	}
	return m
}

// MustCloseMain closes the program. Fail on error.
func MustCloseMain(tb testing.TB, m *main.Main) {
	tb.Helper()
	if err := m.Close(); err != nil {
		tb.Fatal(err)
	}
}

func MustClient(tb testing.TB, host string) *http.Client {
	tb.Helper()
	return http.NewClient(http.WithServer(host))
}
