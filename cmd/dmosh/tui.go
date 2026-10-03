package main

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// runTUI holds the terminal until the user quits. It is what bare `dmosh` does.
func (m *Main) runTUI(ctx context.Context) error {
	p := tea.NewProgram(newModel(m),
		// Streams come from Main so the program is testable, and the context so
		// a SIGINT reaches the renderer and the terminal gets restored.
		tea.WithContext(ctx),
		tea.WithInput(m.Stdin),
		tea.WithOutput(m.Stdout),
	)

	// A user quitting with ctrl+c is how this program ends, not a failure.
	if _, err := p.Run(); err != nil && !errors.Is(err, tea.ErrInterrupted) {
		return err
	}
	return nil
}

// connection is what the model knows about the daemon. It starts unknown, and
// every probe moves it to one of the other two.
type connection int

const (
	connecting connection = iota
	connected
	unreachable
)

// model is the root of the interface. A real dmosh grows panes below this — a
// character list, a sheet view — each its own tea.Model this one delegates to.
// Today it holds the one fact the skeleton fetches.
type model struct {
	main *Main

	width  int
	height int

	conn    connection
	connErr error
}

func newModel(m *Main) model {
	return model{main: m, conn: connecting}
}

// healthMsg carries the result of a probe back into Update. A message per
// request keeps the client call off the Update goroutine, which is the rule the
// rest of the interface will follow when it starts loading characters.
type healthMsg struct{ err error }

// checkHealth probes the daemon in the background. It runs the same
// Main.probeHealth the `dmosh health` subcommand does.
func (m model) checkHealth() tea.Cmd {
	return func() tea.Msg {
		return healthMsg{err: m.main.probeHealth(context.Background())}
	}
}

func (m model) Init() tea.Cmd {
	return m.checkHealth()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case healthMsg:
		if msg.err != nil {
			m.conn, m.connErr = unreachable, msg.err
		} else {
			m.conn, m.connErr = connected, nil
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.conn, m.connErr = connecting, nil
			return m, m.checkHealth()
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	body := strings.Join([]string{
		titleStyle.Render("DMOST"),
		"",
		m.statusLine(),
		"",
		mutedStyle.Render("Nothing is wired to the interface yet — this is a skeleton."),
		"",
		mutedStyle.Render("r reload · q quit"),
	}, "\n")

	v := tea.NewView(body)

	// The alt screen is a property of the view in bubbletea v2, not a program
	// option: the terminal is restored when the program stops rendering.
	v.AltScreen = true
	v.WindowTitle = "dmosh"
	return v
}

// statusLine renders what the model knows about the daemon.
func (m model) statusLine() string {
	switch m.conn {
	case connected:
		return okStyle.Render("● connected") + mutedStyle.Render("  "+m.main.Server)
	case unreachable:
		return errStyle.Render("● unreachable") +
			mutedStyle.Render("  "+m.main.Server) +
			"\n" + mutedStyle.Render(m.connErr.Error())
	default:
		return mutedStyle.Render("○ connecting…  " + m.main.Server)
	}
}

// Styles are package-level because they are constant, and named for their role
// rather than their color so a theme can be swapped underneath them.
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// compile-time proof the root model still satisfies bubbletea's interface.
var _ tea.Model = model{}
