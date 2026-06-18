package tui

import (
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

func TestPresetPrefixCancelDoesNotFire(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)

	m = update(m, runeKey('s'))
	if m.pending != "save" {
		t.Fatalf("after 's', pending=%q want save", m.pending)
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.pending != "" {
		t.Fatalf("after esc, pending=%q want empty", m.pending)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("no device calls expected, got %v", fake.calls)
	}
}

func TestPresetSaveFires(t *testing.T) {
	fake := &fakeMatrix{routes: map[int]int{1: 1, 2: 2, 3: 3, 4: 4}}
	m := newModel(fake)

	m = update(m, runeKey('s'))
	_, cmd := m.Update(runeKey('3')) // save to slot 3
	if cmd == nil {
		t.Fatal("expected a command from save")
	}
	cmd() // execute the action closure -> records "save" on the fake
	found := false
	for _, c := range fake.calls {
		if c == "save" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a save call, got %v", fake.calls)
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
