package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// statusMsg carries the result of a status poll.
type statusMsg struct {
	routes map[int]int
	err    error
}

// tickMsg fires on the auto-refresh interval.
type tickMsg time.Time

// pollCmd reads current routing from the matrix.
func (m Model) pollCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		routes, err := client.Status()
		return statusMsg{routes: routes, err: err}
	}
}

// tickCmd schedules the next auto-refresh.
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.cfg.PollDuration(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

// actionCmd performs a no-reply command then re-polls (fire-then-refresh),
// since switch/preset/buzzer commands return nothing on the wire.
func (m Model) actionCmd(do func() error) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_ = do()
		routes, err := client.Status()
		return statusMsg{routes: routes, err: err}
	}
}

// applyRoutesCmd drives the matrix to the given {output: input} snapshot using
// the minimal command set, then re-polls.
func (m Model) applyRoutesCmd(routes map[int]int) tea.Cmd {
	client := m.client
	nout := m.cfg.NumOutputs()

	// Stable (output, input) pairs for outputs that are set.
	pairs := make([][2]int, 0, len(routes))
	for out := 1; out <= nout; out++ {
		if in, ok := routes[out]; ok && in > 0 {
			pairs = append(pairs, [2]int{out, in})
		}
	}

	return func() tea.Msg {
		switch {
		case len(pairs) == nout && allSameInput(pairs):
			_ = client.AllTo(pairs[0][1])
		case len(pairs) == nout && isMirror(pairs):
			_ = client.Mirror()
		default:
			for _, p := range pairs {
				_ = client.Route(p[1], p[0]) // Route(in, out)
			}
		}
		routes, err := client.Status()
		return statusMsg{routes: routes, err: err}
	}
}

func allSameInput(pairs [][2]int) bool {
	if len(pairs) == 0 {
		return false
	}
	in := pairs[0][1]
	for _, p := range pairs {
		if p[1] != in {
			return false
		}
	}
	return true
}

func isMirror(pairs [][2]int) bool {
	for _, p := range pairs {
		if p[0] != p[1] { // output != input
			return false
		}
	}
	return true
}
