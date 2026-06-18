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
