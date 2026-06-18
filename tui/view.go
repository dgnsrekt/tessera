package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	dotGreen    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	dotRed      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	toastStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	legendStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("4")).Padding(1, 2)
)

// View renders the whole screen.
func (m Model) View() string {
	switch m.mode {
	case modeLabelEdit:
		return m.labelEditView()
	case modeSceneEdit:
		return m.sceneEditView()
	case modeScene:
		return m.sceneScreen()
	}
	var b strings.Builder
	b.WriteString(m.statusBar())
	b.WriteString("\n\n")
	b.WriteString(m.gridView())
	b.WriteString("\n")
	b.WriteString(toastStyle.Render(m.toast))
	b.WriteString("\n\n")
	b.WriteString(m.legend())
	return b.String()
}

func (m Model) statusBar() string {
	dot := dotGreen.Render("●")
	state := "connected"
	if !m.connected {
		dot = dotRed.Render("●")
		state = "disconnected"
	}
	tag := "[GRID]"
	if m.mode == modeScene || m.mode == modeSceneEdit {
		tag = "[SCENES]"
	}
	left := fmt.Sprintf("%s %s  %s",
		dot,
		boldStyle.Render(fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)),
		state,
	)
	right := dimStyle.Render(fmt.Sprintf("buzzer %s   %s   tessera %s", onOff(m.buzzerOn), tag, m.version))
	return left + "    " + right
}

func (m Model) gridView() string {
	nin, nout := m.cfg.NumInputs(), m.cfg.NumOutputs()

	rowLabelW := runeLen("Output")
	for i := 1; i <= nout; i++ {
		rowLabelW = max(rowLabelW, runeLen(m.cfg.OutputLabel(i)))
	}
	colW := make([]int, nin+1)
	for j := 1; j <= nin; j++ {
		colW[j] = max(7, runeLen(m.cfg.InputLabel(j))+2)
	}

	var b strings.Builder

	// Header row: blank row-label cell, then centered input labels.
	b.WriteString(strings.Repeat(" ", rowLabelW))
	b.WriteString("  ")
	for j := 1; j <= nin; j++ {
		b.WriteString(boldStyle.Render(center(m.cfg.InputLabel(j), colW[j])))
		b.WriteString(" ")
	}
	b.WriteString("\n")

	// One row per output.
	for i := 1; i <= nout; i++ {
		b.WriteString(boldStyle.Render(padRight(m.cfg.OutputLabel(i), rowLabelW)))
		b.WriteString("  ")
		for j := 1; j <= nin; j++ {
			glyph := "·"
			active := m.routes[i] == j
			if active {
				glyph = "●"
			}
			field := center(glyph, colW[j])
			style := lipgloss.NewStyle()
			if active {
				style = greenStyle
			} else {
				style = dimStyle
			}
			if i-1 == m.cursorRow && j-1 == m.cursorCol {
				style = style.Reverse(true)
			}
			b.WriteString(style.Render(field))
			b.WriteString(" ")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) legend() string {
	return legendStyle.Render(
		"↑↓←→ move   enter route   1-4 all→input   m mirror   " +
			"tab scenes   b buzzer   e labels   R refresh   q quit",
	)
}

func (m Model) sceneLegend() string {
	return legendStyle.Render(
		"↑↓ select   enter apply   n new   s update   e edit   d delete   tab/esc back   q quit",
	)
}

func (m Model) sceneScreen() string {
	var b strings.Builder
	b.WriteString(m.statusBar())
	b.WriteString("\n\n")
	b.WriteString(boldStyle.Render("Scenes"))
	b.WriteString("\n\n")

	if len(m.cfg.Scenes) == 0 {
		b.WriteString(dimStyle.Render("No scenes yet — press "))
		b.WriteString(boldStyle.Render("n"))
		b.WriteString(dimStyle.Render(" to capture the current routing as a scene."))
		b.WriteString("\n\n")
		b.WriteString(toastStyle.Render(m.toast))
		b.WriteString("\n\n")
		b.WriteString(m.sceneLegend())
		return b.String()
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.sceneList(), "   ", m.sceneDetail()))
	b.WriteString("\n\n")
	b.WriteString(toastStyle.Render(m.toast))
	b.WriteString("\n\n")
	b.WriteString(m.sceneLegend())
	return b.String()
}

func (m Model) sceneList() string {
	var b strings.Builder
	curr := m.routes
	for i, sc := range m.cfg.Scenes {
		marker := " "
		if routesEqual(sc.RoutesMap(m.cfg.NumOutputs()), curr) {
			marker = greenStyle.Render("●")
		}
		badge := ""
		if sc.Slot > 0 {
			badge = dimStyle.Render(fmt.Sprintf(" [%d]", sc.Slot))
		}
		name := sc.Name
		if name == "" {
			name = fmt.Sprintf("Scene %d", i+1)
		}
		row := fmt.Sprintf("%s %s%s", marker, name, badge)
		if i == m.sceneSel {
			row = lipgloss.NewStyle().Reverse(true).Render(fmt.Sprintf("%s %s%s", marker, name, badge))
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(28).Render(b.String())
}

func (m Model) sceneDetail() string {
	if m.sceneSel < 0 || m.sceneSel >= len(m.cfg.Scenes) {
		return ""
	}
	sc := m.cfg.Scenes[m.sceneSel]
	var b strings.Builder
	name := sc.Name
	if name == "" {
		name = fmt.Sprintf("Scene %d", m.sceneSel+1)
	}
	b.WriteString(boldStyle.Render(name))
	if sc.Slot > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("   hardware preset %d", sc.Slot)))
	}
	b.WriteString("\n")
	if sc.Description != "" {
		b.WriteString(dimStyle.Render(sc.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	rm := sc.RoutesMap(m.cfg.NumOutputs())
	for out := 1; out <= m.cfg.NumOutputs(); out++ {
		in, ok := rm[out]
		line := fmt.Sprintf("%s → ", m.cfg.OutputLabel(out))
		if ok {
			line += m.cfg.InputLabel(in)
		} else {
			line += dimStyle.Render("(unset)")
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return boxStyle.Render(b.String())
}

func (m Model) sceneEditView() string {
	verb := "New scene"
	if m.editingScene >= 0 {
		verb = "Edit scene"
	}
	slotStr := "none"
	if m.sceneSlot > 0 {
		slotStr = fmt.Sprintf("%d", m.sceneSlot)
	}
	slotLine := fmt.Sprintf("Hardware slot: ◄ %s ►", slotStr)
	if m.sceneEditFocus == 2 {
		slotLine = lipgloss.NewStyle().Reverse(true).Render(slotLine)
	}
	body := fmt.Sprintf(
		"%s\n\nName:\n%s\n\nDescription:\n%s\n\n%s\n\n%s",
		boldStyle.Render(verb),
		m.sceneName.View(),
		m.sceneDesc.View(),
		slotLine,
		dimStyle.Render("tab/↑↓ move · ←→ change slot · enter save · esc cancel"),
	)
	box := boxStyle.Render(body)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

func (m Model) labelEditView() string {
	title := boldStyle.Render(fmt.Sprintf("Edit labels  (Input %d / Output %d)", m.editIn, m.editOut))
	body := fmt.Sprintf(
		"%s\n\nInput %d name:\n%s\n\nOutput %d name:\n%s\n\n%s",
		title,
		m.editIn, m.inInput.View(),
		m.editOut, m.outInput.View(),
		dimStyle.Render("tab switch · enter save · esc cancel"),
	)
	box := boxStyle.Render(body)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// center pads s with spaces to width w, biasing extra space to the right.
func center(s string, w int) string {
	n := runeLen(s)
	if n >= w {
		return s
	}
	left := (w - n) / 2
	right := w - n - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func padRight(s string, w int) string {
	n := runeLen(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}
