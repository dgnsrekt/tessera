// Package tui is the Bubble Tea terminal UI for the tessera matrix controller.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dgnsrekt/tessera/internal/config"
	"github.com/dgnsrekt/tessera/internal/matrix"
)

type mode int

const (
	modeGrid mode = iota
	modeLabelEdit
)

// Model is the application state.
type Model struct {
	cfg     config.Config
	client  matrix.Matrix
	version string

	routes    map[int]int
	cursorRow int // 0-based output index
	cursorCol int // 0-based input index

	connected bool
	pending   string // "", "save", or "recall" (awaiting a digit)
	buzzerOn  bool
	toast     string

	mode      mode
	inInput   textinput.Model
	outInput  textinput.Model
	editFocus int // 0 = input field, 1 = output field
	editIn    int
	editOut   int

	width  int
	height int
}

// New builds the initial model.
func New(cfg config.Config, client matrix.Matrix, version string) Model {
	in := textinput.New()
	in.CharLimit = 32
	out := textinput.New()
	out.CharLimit = 32
	return Model{
		cfg:      cfg,
		client:   client,
		version:  version,
		routes:   map[int]int{},
		mode:     modeGrid,
		inInput:  in,
		outInput: out,
	}
}

// Init kicks off the first poll and the refresh ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.pollCmd(), m.tickCmd())
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case statusMsg:
		if msg.err == nil && msg.routes != nil {
			m.routes = msg.routes
			m.connected = true
		} else {
			m.connected = false
		}
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.pollCmd(), m.tickCmd())
	case tea.KeyMsg:
		if m.mode == modeLabelEdit {
			return m.updateLabelEdit(msg)
		}
		return m.updateGrid(msg)
	}
	return m, nil
}

func (m Model) updateGrid(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	nin, nout := m.cfg.NumInputs(), m.cfg.NumOutputs()

	// Two-key preset sequence: 's'/'r' then a digit 1-8.
	if m.pending != "" {
		switch {
		case key == "esc":
			m.pending = ""
			m.toast = "cancelled"
			return m, nil
		case isDigit(key, 1, 8):
			slot := int(key[0] - '0')
			pending := m.pending
			m.pending = ""
			if pending == "save" {
				m.toast = fmt.Sprintf("saved current routing → preset %d", slot)
				return m, m.actionCmd(func() error { return m.client.SavePreset(slot) })
			}
			m.toast = fmt.Sprintf("recalled preset %d", slot)
			return m, m.actionCmd(func() error { return m.client.RecallPreset(slot) })
		default:
			m.pending = "" // any other key cancels the prefix
		}
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursorRow > 0 {
			m.cursorRow--
		}
	case "down", "j":
		if m.cursorRow < nout-1 {
			m.cursorRow++
		}
	case "left", "h":
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case "right", "l":
		if m.cursorCol < nin-1 {
			m.cursorCol++
		}
	case "enter", " ":
		in, out := m.cursorCol+1, m.cursorRow+1
		m.toast = fmt.Sprintf("routed %s → %s", m.cfg.InputLabel(in), m.cfg.OutputLabel(out))
		return m, m.actionCmd(func() error { return m.client.Route(in, out) })
	case "m":
		m.toast = "mirrored each output to its same-numbered input"
		return m, m.actionCmd(func() error { return m.client.Mirror() })
	case "s":
		m.pending = "save"
		m.toast = "save to preset slot? press 1-8 (esc cancels)"
	case "r":
		m.pending = "recall"
		m.toast = "recall preset slot? press 1-8 (esc cancels)"
	case "b":
		m.buzzerOn = !m.buzzerOn
		on := m.buzzerOn
		m.toast = "buzzer " + onOff(on)
		return m, m.actionCmd(func() error { return m.client.Buzzer(on) })
	case "e":
		m.enterLabelEdit()
		return m, textinput.Blink
	case "R":
		m.toast = "refreshed"
		return m, m.pollCmd()
	default:
		// digit 1..numInputs -> send that input to all outputs
		if isDigit(key, 1, 9) {
			n := int(key[0] - '0')
			if n <= nin {
				m.toast = fmt.Sprintf("all outputs → %s", m.cfg.InputLabel(n))
				return m, m.actionCmd(func() error { return m.client.AllTo(n) })
			}
		}
	}
	return m, nil
}

func (m *Model) enterLabelEdit() {
	m.editIn = m.cursorCol + 1
	m.editOut = m.cursorRow + 1
	m.inInput.SetValue(m.cfg.InputLabel(m.editIn))
	m.outInput.SetValue(m.cfg.OutputLabel(m.editOut))
	m.editFocus = 0
	m.inInput.Focus()
	m.outInput.Blur()
	m.mode = modeLabelEdit
}

func (m Model) updateLabelEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeGrid
		m.toast = "cancelled"
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.editFocus = 1 - m.editFocus
		if m.editFocus == 0 {
			m.inInput.Focus()
			m.outInput.Blur()
		} else {
			m.outInput.Focus()
			m.inInput.Blur()
		}
		return m, textinput.Blink
	case "enter":
		inName := strings.TrimSpace(m.inInput.Value())
		outName := strings.TrimSpace(m.outInput.Value())
		if inName == "" {
			inName = fmt.Sprintf("Input %d", m.editIn)
		}
		if outName == "" {
			outName = fmt.Sprintf("Output %d", m.editOut)
		}
		m.cfg.Inputs[m.editIn-1] = inName
		m.cfg.Outputs[m.editOut-1] = outName
		_ = config.Save(m.cfg)
		m.mode = modeGrid
		m.toast = "labels saved"
		return m, nil
	}

	var cmd tea.Cmd
	if m.editFocus == 0 {
		m.inInput, cmd = m.inInput.Update(msg)
	} else {
		m.outInput, cmd = m.outInput.Update(msg)
	}
	return m, cmd
}

// isDigit reports whether key is a single digit character within [lo, hi].
func isDigit(key string, lo, hi int) bool {
	if len(key) != 1 || key[0] < '0' || key[0] > '9' {
		return false
	}
	n := int(key[0] - '0')
	return n >= lo && n <= hi
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
