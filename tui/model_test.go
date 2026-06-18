package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dgnsrekt/tessera/internal/config"
)

// fakeMatrix records calls instead of touching a device.
type fakeMatrix struct {
	routes map[int]int
	calls  []string
}

func (f *fakeMatrix) Status() (map[int]int, error) { return f.routes, nil }
func (f *fakeMatrix) Route(in, out int) error {
	f.calls = append(f.calls, "route")
	f.routes[out] = in
	return nil
}
func (f *fakeMatrix) AllTo(in int) error       { f.calls = append(f.calls, "allto"); return nil }
func (f *fakeMatrix) Mirror() error            { f.calls = append(f.calls, "mirror"); return nil }
func (f *fakeMatrix) SavePreset(p int) error   { f.calls = append(f.calls, "save"); return nil }
func (f *fakeMatrix) RecallPreset(p int) error { f.calls = append(f.calls, "recall"); return nil }
func (f *fakeMatrix) Buzzer(on bool) error     { f.calls = append(f.calls, "buzzer"); return nil }
func (f *fakeMatrix) Connected() bool          { return true }

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func update(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		m = update(m, runeKey(r))
	}
	return m
}

func (f *fakeMatrix) called(name string) bool {
	return slices.Contains(f.calls, name)
}

func newModel(fake *fakeMatrix) Model {
	m := New(config.Default(), fake, "test")
	m = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = update(m, statusMsg{routes: fake.routes}) // simulate first poll
	return m
}

func TestGridRendersDiagonal(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)
	view := m.View()
	for _, want := range []string{"Input 1", "Output 1", "●", "connected"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestTabTogglesSceneMode(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)

	m = update(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.mode != modeScene {
		t.Fatalf("after tab, mode=%v want scene", m.mode)
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.mode != modeGrid {
		t.Fatalf("after second tab, mode=%v want grid", m.mode)
	}
}

func TestNewSceneCapturesAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fake := &fakeMatrix{routes: map[int]int{1: 2, 2: 2, 3: 1, 4: 1}}
	m := newModel(fake)

	m = update(m, tea.KeyMsg{Type: tea.KeyTab}) // -> scene view
	m = update(m, runeKey('n'))                 // -> editor (capture current)
	if m.mode != modeSceneEdit {
		t.Fatalf("after 'n', mode=%v want sceneEdit", m.mode)
	}
	m = typeText(m, "Movie Night")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter}) // save

	if len(m.cfg.Scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(m.cfg.Scenes))
	}
	sc := m.cfg.Scenes[0]
	if sc.Name != "Movie Night" {
		t.Fatalf("name=%q want Movie Night", sc.Name)
	}
	want := []int{2, 2, 1, 1}
	if len(sc.Routes) != 4 || sc.Routes[0] != want[0] || sc.Routes[3] != want[3] {
		t.Fatalf("routes=%v want %v", sc.Routes, want)
	}
	// persisted to temp config
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Scenes) != 1 || saved.Scenes[0].Name != "Movie Night" {
		t.Fatalf("persisted scenes=%v", saved.Scenes)
	}
}

func TestApplySceneReplaysRouting(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)
	m.cfg.Scenes = []config.Scene{{Name: "All2", Routes: []int{2, 2, 2, 2}}}

	m = update(m, tea.KeyMsg{Type: tea.KeyTab}) // scene view
	_, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected an apply command")
	}
	cmd() // run the replay closure
	if !fake.called("allto") {
		t.Fatalf("expected an AllTo replay (all outputs same input), calls=%v", fake.calls)
	}
}

func TestSceneSlotWritesHardwarePreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)

	m = update(m, tea.KeyMsg{Type: tea.KeyTab})
	m = update(m, runeKey('n'))
	m = typeText(m, "Mirror")
	m.sceneSlot = 1 // assign hardware slot
	_, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command (hardware preset write)")
	}
	cmd()
	if !fake.called("save") {
		t.Fatalf("expected a SavePreset call for slot 1, calls=%v", fake.calls)
	}
}

func TestDeleteSceneWithConfirm(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)
	m.cfg.Scenes = []config.Scene{{Name: "Gone", Routes: []int{1, 2, 3, 4}}}

	m = update(m, tea.KeyMsg{Type: tea.KeyTab})
	m = update(m, runeKey('d'))
	if !m.confirmDelete {
		t.Fatal("expected confirmDelete armed")
	}
	m = update(m, runeKey('n')) // cancel
	if len(m.cfg.Scenes) != 1 {
		t.Fatalf("cancel should keep scene, got %d", len(m.cfg.Scenes))
	}
	m = update(m, runeKey('d'))
	m = update(m, runeKey('y')) // confirm
	if len(m.cfg.Scenes) != 0 {
		t.Fatalf("confirm should delete, got %d", len(m.cfg.Scenes))
	}
}

func TestSceneViewShowsNameAndPreview(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)
	m.cfg.Inputs[1] = "Apple TV"
	m.cfg.Scenes = []config.Scene{{Name: "Living", Description: "den setup", Routes: []int{2, 2, 2, 2}}}
	m = update(m, tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	for _, want := range []string{"Living", "den setup", "Apple TV", "[SCENES]"} {
		if !strings.Contains(view, want) {
			t.Errorf("scene view missing %q\n---\n%s", want, view)
		}
	}
}

func TestLabelEditPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)

	m = update(m, runeKey('e'))
	if m.mode != modeLabelEdit {
		t.Fatalf("after 'e', mode=%v want labelEdit", m.mode)
	}
	// clear the prefilled value and type a new name
	m.inInput.SetValue("")
	for _, r := range "Apple TV" {
		m = update(m, runeKey(r))
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeGrid {
		t.Fatalf("after enter, mode=%v want grid", m.mode)
	}
	if m.cfg.Inputs[0] != "Apple TV" {
		t.Fatalf("input label = %q want Apple TV", m.cfg.Inputs[0])
	}
	// header rebuilt with the new label
	if !strings.Contains(m.View(), "Apple TV") {
		t.Error("view does not show new label")
	}
	// persisted to the temp config
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Inputs[0] != "Apple TV" {
		t.Fatalf("persisted input = %q want Apple TV", saved.Inputs[0])
	}
}
