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
	if m.mode == modeLabelEdit {
		return m.labelEditView()
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
	left := fmt.Sprintf("%s %s  %s",
		dot,
		boldStyle.Render(fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)),
		state,
	)
	right := dimStyle.Render(fmt.Sprintf("buzzer %s   tessera %s", onOff(m.buzzerOn), m.version))
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
			"s+1-8 save   r+1-8 recall   b buzzer   e labels   R refresh   q quit",
	)
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
