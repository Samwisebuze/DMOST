package main

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/http"
)

// MustRunDaemon starts a real dmostd stack in-process on a random port, so
// these tests exercise the same wire path the binary uses rather than a stub of
// it. It mirrors cmd/dmostd's own Main.Run; the two module graphs do not meet,
// so the wiring is repeated rather than imported.
func MustRunDaemon(tb testing.TB) *http.Server {
	tb.Helper()

	srv := http.NewServer()
	srv.Addr = ":0"
	srv.SetApp(app.App{
		UserService:      &services.UserService{Users: inmem.NewUserRepository()},
		CharacterService: &services.CharacterService{Characters: inmem.NewCharacterRepository()},
	})

	require.NoError(tb, srv.Open())
	tb.Cleanup(func() { require.NoError(tb, srv.Close()) })

	return srv
}

// MustRunMain drives the program the way a shell would — argv in, streams out —
// and hands back what it wrote.
func MustRunMain(tb testing.TB, args ...string) (m *Main, stdout, stderr *bytes.Buffer, err error) {
	tb.Helper()

	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	m = NewMain()
	m.Stdin = strings.NewReader("")
	m.Stdout = stdout
	m.Stderr = stderr

	return m, stdout, stderr, m.Run(context.Background(), args)
}

// deadAddr returns a URL nothing is listening on: a port is bound to reserve it
// and released immediately, which is as close to certain as this gets.
func deadAddr(tb testing.TB) string {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(tb, err)
	require.NoError(tb, ln.Close())

	return "http://" + ln.Addr().String()
}

func TestHealth_Reachable(t *testing.T) {
	t.Parallel()

	srv := MustRunDaemon(t)

	_, stdout, _, err := MustRunMain(t, "health", "--server", srv.URL())
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), srv.URL()+": ok")
}

func TestHealth_Unreachable(t *testing.T) {
	t.Parallel()

	addr := deadAddr(t)

	_, _, _, err := MustRunMain(t, "health", "--server", addr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), addr)
}

func TestVersion(t *testing.T) {
	t.Parallel()

	_, stdout, _, err := MustRunMain(t, "version")
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(stdout.String()))
}

// An unknown subcommand must fail rather than silently opening the TUI, which
// would hang a script.
func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	_, _, _, err := MustRunMain(t, "summon-dragon")
	require.Error(t, err)
}

// The TUI half of the skeleton, driven through Update directly: a program with
// a real terminal is not something a unit test should need.
func TestModel_HealthProbeUpdatesStatus(t *testing.T) {
	t.Parallel()

	srv := MustRunDaemon(t)

	m := NewMain()
	m.Server = srv.URL()
	m.Client = http.NewClient(http.WithServer(m.Server))

	mdl := newModel(m)
	assert.Contains(t, mdl.View().Content, "connecting")

	// Init's command is what a running program would call on the message pump.
	msg := mdl.Init()()
	require.IsType(t, healthMsg{}, msg)

	updated, _ := mdl.Update(msg)
	assert.Contains(t, updated.View().Content, "connected")
}

func TestModel_QuitsOnQ(t *testing.T) {
	t.Parallel()

	mdl := newModel(NewMain())

	_, cmd := mdl.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}
