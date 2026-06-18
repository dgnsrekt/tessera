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
	modeScene
	modeSceneEdit
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
	buzzerOn  bool
	toast     string

	mode      mode
	inInput   textinput.Model
	outInput  textinput.Model
	editFocus int // 0 = input field, 1 = output field
	editIn    int
	editOut   int

	// Scene view / editor state.
	sceneSel       int  // selected scene index
	confirmDelete  bool // armed by 'd', resolved by y/n
	editingScene   int  // index being edited, or -1 when capturing a new scene
	sceneName      textinput.Model
	sceneDesc      textinput.Model
	sceneSlot      int // 0 = none, 1-8 = hardware preset
	sceneEditFocus int // 0 = name, 1 = description, 2 = slot

	width  int
	height int
}

// New builds the initial model.
func New(cfg config.Config, client matrix.Matrix, version string) Model {
	in := textinput.New()
	in.CharLimit = 32
	out := textinput.New()
	out.CharLimit = 32
	name := textinput.New()
	name.CharLimit = 32
	desc := textinput.New()
	desc.CharLimit = 80
	return Model{
		cfg:       cfg,
		client:    client,
		version:   version,
		routes:    map[int]int{},
		mode:      modeGrid,
		inInput:   in,
		outInput:  out,
		sceneName: name,
		sceneDesc: desc,
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
		switch m.mode {
		case modeLabelEdit:
			return m.updateLabelEdit(msg)
		case modeScene:
			return m.updateScene(msg)
		case modeSceneEdit:
			return m.updateSceneEdit(msg)
		default:
			return m.updateGrid(msg)
		}
	}
	return m, nil
}

func (m Model) updateGrid(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	nin, nout := m.cfg.NumInputs(), m.cfg.NumOutputs()

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.enterSceneView()
		return m, nil
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

// -- scene view ---------------------------------------------------------

func (m *Model) enterSceneView() {
	m.mode = modeScene
	m.confirmDelete = false
	if m.sceneSel >= len(m.cfg.Scenes) {
		m.sceneSel = 0
	}
}

func (m Model) updateScene(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	n := len(m.cfg.Scenes)

	if m.confirmDelete {
		switch key {
		case "y":
			return m.deleteSelectedScene()
		case "n", "esc":
			m.confirmDelete = false
			m.toast = "delete cancelled"
		}
		return m, nil
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "esc":
		m.mode = modeGrid
	case "up", "k":
		if m.sceneSel > 0 {
			m.sceneSel--
		}
	case "down", "j":
		if m.sceneSel < n-1 {
			m.sceneSel++
		}
	case "enter", " ":
		if n == 0 {
			return m, nil
		}
		sc := m.cfg.Scenes[m.sceneSel]
		m.toast = "applied scene: " + sc.Name
		return m, m.applyRoutesCmd(sc.RoutesMap(m.cfg.NumOutputs()))
	case "n":
		m.beginNewScene()
		return m, textinput.Blink
	case "s":
		if n == 0 {
			return m, nil
		}
		return m.updateSceneFromCurrent()
	case "e":
		if n == 0 {
			return m, nil
		}
		m.beginEditScene()
		return m, textinput.Blink
	case "d":
		if n == 0 {
			return m, nil
		}
		m.confirmDelete = true
		m.toast = fmt.Sprintf("delete %q? y/n", m.cfg.Scenes[m.sceneSel].Name)
	}
	return m, nil
}

// updateSceneFromCurrent re-captures the selected scene's routing from the live
// state and, if it has a hardware slot, rewrites that preset too.
func (m Model) updateSceneFromCurrent() (tea.Model, tea.Cmd) {
	m.cfg.Scenes[m.sceneSel].Routes = m.currentRoutesSlice()
	_ = config.Save(m.cfg)
	sc := m.cfg.Scenes[m.sceneSel]
	m.toast = "updated scene: " + sc.Name
	if sc.Slot > 0 {
		return m, m.actionCmd(func() error { return m.client.SavePreset(sc.Slot) })
	}
	return m, nil
}

func (m Model) deleteSelectedScene() (tea.Model, tea.Cmd) {
	idx := m.sceneSel
	name := m.cfg.Scenes[idx].Name
	next := append([]config.Scene{}, m.cfg.Scenes[:idx]...)
	m.cfg.Scenes = append(next, m.cfg.Scenes[idx+1:]...)
	if m.sceneSel >= len(m.cfg.Scenes) && m.sceneSel > 0 {
		m.sceneSel--
	}
	_ = config.Save(m.cfg)
	m.confirmDelete = false
	m.toast = "deleted scene: " + name
	return m, nil
}

// -- scene editor -------------------------------------------------------

func (m *Model) beginNewScene() {
	m.editingScene = -1 // capture current routing
	m.sceneName.SetValue("")
	m.sceneDesc.SetValue("")
	m.sceneSlot = 0
	m.sceneEditFocus = 0
	m.syncSceneEditFocus()
	m.mode = modeSceneEdit
}

func (m *Model) beginEditScene() {
	sc := m.cfg.Scenes[m.sceneSel]
	m.editingScene = m.sceneSel
	m.sceneName.SetValue(sc.Name)
	m.sceneDesc.SetValue(sc.Description)
	m.sceneSlot = sc.Slot
	m.sceneEditFocus = 0
	m.syncSceneEditFocus()
	m.mode = modeSceneEdit
}

func (m *Model) syncSceneEditFocus() {
	if m.sceneEditFocus == 0 {
		m.sceneName.Focus()
	} else {
		m.sceneName.Blur()
	}
	if m.sceneEditFocus == 1 {
		m.sceneDesc.Focus()
	} else {
		m.sceneDesc.Blur()
	}
}

func (m Model) updateSceneEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeScene
		m.toast = "cancelled"
		return m, nil
	case "enter":
		return m.saveSceneEdit()
	case "tab", "down":
		m.sceneEditFocus = (m.sceneEditFocus + 1) % 3
		m.syncSceneEditFocus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.sceneEditFocus = (m.sceneEditFocus + 2) % 3
		m.syncSceneEditFocus()
		return m, textinput.Blink
	case "left":
		if m.sceneEditFocus == 2 {
			if m.sceneSlot > 0 {
				m.sceneSlot--
			}
			return m, nil
		}
	case "right":
		if m.sceneEditFocus == 2 {
			if m.sceneSlot < 8 {
				m.sceneSlot++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.sceneEditFocus {
	case 0:
		m.sceneName, cmd = m.sceneName.Update(msg)
	case 1:
		m.sceneDesc, cmd = m.sceneDesc.Update(msg)
	}
	return m, cmd
}

func (m Model) saveSceneEdit() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.sceneName.Value())
	if name == "" {
		name = fmt.Sprintf("Scene %d", len(m.cfg.Scenes)+1)
	}
	desc := strings.TrimSpace(m.sceneDesc.Value())
	slot := m.sceneSlot

	if m.editingScene < 0 {
		// New scene captured from the current live routing.
		m.cfg.Scenes = append(m.cfg.Scenes, config.Scene{
			Name:        name,
			Description: desc,
			Routes:      m.currentRoutesSlice(),
			Slot:        slot,
		})
		m.sceneSel = len(m.cfg.Scenes) - 1
	} else {
		m.cfg.Scenes[m.editingScene].Name = name
		m.cfg.Scenes[m.editingScene].Description = desc
		m.cfg.Scenes[m.editingScene].Slot = slot
	}
	_ = config.Save(m.cfg)
	m.mode = modeScene
	m.toast = "saved scene: " + name

	// Mirror to a hardware preset only when the snapshot matches live routing
	// right now (always true for a fresh capture).
	idx := m.editingScene
	if idx < 0 {
		idx = len(m.cfg.Scenes) - 1
	}
	sc := m.cfg.Scenes[idx]
	if sc.Slot > 0 && routesEqual(sc.RoutesMap(m.cfg.NumOutputs()), m.routes) {
		return m, m.actionCmd(func() error { return m.client.SavePreset(sc.Slot) })
	}
	return m, nil
}

// currentRoutesSlice snapshots the live routing as a []int indexed by output-1.
func (m Model) currentRoutesSlice() []int {
	n := m.cfg.NumOutputs()
	r := make([]int, n)
	for out := 1; out <= n; out++ {
		r[out-1] = m.routes[out] // 0 if unset
	}
	return r
}

func routesEqual(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
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
