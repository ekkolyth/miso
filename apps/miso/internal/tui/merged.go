package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Fixed color palette for workspace labels — cycles if more workspaces than colors.
var labelColors = []color.Color{
	lipgloss.Color("#7c3aed"), // purple
	lipgloss.Color("#3b82f6"), // blue
	lipgloss.Color("#f59e0b"), // amber
	lipgloss.Color("#10b981"), // emerald
	lipgloss.Color("#ef4444"), // red
	lipgloss.Color("#ec4899"), // pink
	lipgloss.Color("#06b6d4"), // cyan
	lipgloss.Color("#f97316"), // orange
}

type MergedModel struct {
	pm               *ProcessManager
	keys             MergedKeyMap
	cursor           int
	visible          map[int]bool
	logLines         []mergedLine
	scrollOffset     int // 0 = pinned to bottom, >0 = scrolled up N lines
	width            int
	height           int
	script           string
	delegated        bool
	allExitedPending bool
	sel              SelectionState
}

type mergedLine struct {
	label string
	color color.Color
	text  string
}

func NewMergedModel(pm *ProcessManager, script string, delegated bool) MergedModel {
	visible := make(map[int]bool)
	for i := range pm.Processes {
		visible[i] = true
	}

	return MergedModel{
		pm:        pm,
		keys:      DefaultMergedKeyMap(),
		visible:   visible,
		script:    script,
		delegated: delegated,
	}
}

func (m MergedModel) Init() tea.Cmd {
	return nil
}

func (m MergedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Left):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Right):
			if m.cursor < len(m.pm.Processes)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Toggle):
			m.visible[m.cursor] = !m.visible[m.cursor]
		case key.Matches(msg, m.keys.Restart):
			if !m.delegated && m.cursor < len(m.pm.Processes) {
				m.allExitedPending = false
				go m.pm.Restart(m.pm.Processes[m.cursor])
			}
		case key.Matches(msg, m.keys.RestartAll):
			m.allExitedPending = false
			go m.pm.RestartAll()
		case key.Matches(msg, m.keys.CopyKey):
			if m.sel.active {
				return m, tea.SetClipboard(m.selectedText())
			}
		case msg.String() == "esc":
			m.sel = SelectionState{}
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			row := m.mouseToLogRow(msg.X, msg.Y)
			if row >= 0 {
				m.sel = SelectionState{active: true, startRow: row, endRow: row}
			}
		}

	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft && m.sel.active {
			row := m.mouseToLogRow(msg.X, msg.Y)
			if row >= 0 {
				m.sel.endRow = row
			}
		}

	case tea.MouseReleaseMsg:
		// selection remains until cleared with esc or new click

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			maxScroll := len(m.logLines) - m.logHeight()
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollOffset += 3
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
			return m, nil
		case tea.MouseWheelDown:
			m.scrollOffset -= 3
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		}

	case ProcessOutputMsg:
		// Default new dynamically-added processes to visible (turbo/nx mode).
		m.ensureVisible()
		color := m.colorForLabel(msg.Label)
		m.logLines = append(m.logLines, mergedLine{
			label: msg.Label,
			color: color,
			text:  msg.Line,
		})
		// Cap merged log to prevent unbounded memory growth
		maxMergedLines := DefaultBufferSize * len(m.pm.Processes)
		if len(m.logLines) > maxMergedLines {
			m.logLines = m.logLines[len(m.logLines)-maxMergedLines:]
		}
		return m, nil

	case ProcessStateMsg:
		m.ensureVisible()
		if !m.allExitedPending && m.pm.AllExited() {
			m.allExitedPending = true
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return allExitedMsg{}
			})
		}
		return m, nil

	case allExitedMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m MergedModel) logHeight() int {
	// header (1) + tab row (1) + selector/divider (1), rest is logs
	h := m.height - 3
	if h < 0 {
		return 0
	}
	return h
}

func (m MergedModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	filterBar := m.renderFilterBar()
	logHeight := m.logHeight()

	logOutput := m.buildLogVisualRows(logHeight)
	// Pad to fill.
	for len(logOutput) < logHeight {
		logOutput = append(logOutput, "")
	}

	// Highlight selected rows.
	for i := range logOutput {
		if m.sel.active && i >= m.sel.minRow() && i <= m.sel.maxRow() {
			logOutput[i] = selectedBg.Render(logOutput[i])
		}
	}

	logContent := strings.Join(logOutput, "\n")

	logPanel := lipgloss.NewStyle().
		Width(m.width).
		MaxHeight(logHeight).
		Padding(0, 1).
		Render(logContent)

	v := tea.NewView(filterBar + "\n" + logPanel)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m MergedModel) renderFilterBar() string {
	// Header bar: "miso [script]" on left, controls on right
	title := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("miso") +
		" " +
		lipgloss.NewStyle().Foreground(mutedColor).Render(m.script)

	scrollHint := ""
	if m.scrollOffset > 0 {
		scrollHint = lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("(scrolled +%d) ", m.scrollOffset))
	}

	var hintText string
	if m.delegated {
		hintText = "←→ select · space toggle · c copy · R restart · ctrl+c quit"
	} else {
		hintText = "←→ select · space toggle · c copy · r restart · R restart all · ctrl+c quit"
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(hintText)

	titleLen := lipgloss.Width(title)
	hintsLen := lipgloss.Width(hints) + lipgloss.Width(scrollHint)
	gap := m.width - titleLen - hintsLen - 2
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(title + strings.Repeat(" ", gap) + scrollHint + hints)

	// Tabs: single-line colored blocks
	var items []string
	var selectorParts []string
	for i, proc := range m.pm.Processes {
		color := labelColors[i%len(labelColors)]
		label := proc.Entry.Label

		if proc.State == StateExited && proc.ExitCode != 0 {
			color = exitedColor
		}

		style := lipgloss.NewStyle().Padding(0, 1).Bold(true)

		if !m.visible[i] {
			style = style.
				Background(lipgloss.Color("#444444")).
				Foreground(lipgloss.Color("#888888")).
				Strikethrough(true)
		} else {
			style = style.
				Background(color).
				Foreground(lipgloss.Color("#ffffff"))
		}

		rendered := style.Render(label)
		items = append(items, rendered)

		// Build selector line: ▔▔▔ under selected tab, spaces under others
		tabWidth := lipgloss.Width(rendered)
		if i == m.cursor {
			selectorParts = append(selectorParts, lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(strings.Repeat("▔", tabWidth)))
		} else {
			selectorParts = append(selectorParts, strings.Repeat(" ", tabWidth))
		}
	}

	tabs := strings.Join(items, " ")
	// Add single-space gaps to selector to match tab spacing
	selector := strings.Join(selectorParts, " ")

	tabRow := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(tabs)

	selectorRow := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(selector)

	return header + "\n" + tabRow + "\n" + selectorRow
}

// ensureVisible sets any newly-added processes (from delegated mode dynamic
// discovery) to visible by default.
func (m MergedModel) ensureVisible() {
	for i := range m.pm.Processes {
		if _, ok := m.visible[i]; !ok {
			m.visible[i] = true
		}
	}
}

func (m MergedModel) colorForLabel(label string) color.Color {
	for i, proc := range m.pm.Processes {
		if proc.Entry.Label == label {
			return labelColors[i%len(labelColors)]
		}
	}
	return lipgloss.Color("#888888")
}

func (m MergedModel) visibleLabels() map[string]bool {
	result := make(map[string]bool)
	for i, proc := range m.pm.Processes {
		if m.visible[i] {
			result[proc.Entry.Label] = true
		}
	}
	return result
}

// mouseToLogRow converts absolute terminal coordinates to a 0-based visual
// log row index. Returns -1 if the coordinate is above the log area.
func (m MergedModel) mouseToLogRow(x, y int) int {
	logPanelTop := 3 // row 0=header, 1=tabs, 2=selector, 3+=logs
	row := y - logPanelTop
	if row < 0 {
		return -1
	}
	return row
}

// selectedText returns the selected log rows as a plain string.
func (m MergedModel) selectedText() string {
	logHeight := m.logHeight()
	logOutput := m.buildLogVisualRows(logHeight)

	// Slice selection.
	lo := m.sel.minRow()
	hi := m.sel.maxRow()
	if lo < 0 {
		lo = 0
	}
	if hi >= len(logOutput) {
		hi = len(logOutput) - 1
	}
	if lo > hi || len(logOutput) == 0 {
		return ""
	}
	return strings.Join(logOutput[lo:hi+1], "\n")
}

// buildLogVisualRows re-derives the visual rows for the merged log panel,
// applying the scroll window, wrapping, and tail-slice.
func (m MergedModel) buildLogVisualRows(logHeight int) []string {
	visibleLabels := m.visibleLabels()
	var visibleLines []mergedLine
	for _, line := range m.logLines {
		if visibleLabels[line.label] {
			visibleLines = append(visibleLines, line)
		}
	}

	totalVisible := len(visibleLines)
	endIdx := totalVisible - m.scrollOffset
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - logHeight
	if startIdx < 0 {
		startIdx = 0
	}
	windowLines := visibleLines[startIdx:endIdx]

	maxLabel := 0
	for _, proc := range m.pm.Processes {
		if len(proc.Entry.Label) > maxLabel {
			maxLabel = len(proc.Entry.Label)
		}
	}

	prefixWidth := maxLabel + 1
	textWidth := m.width - prefixWidth - 2
	if textWidth < 1 {
		textWidth = 1
	}

	var logOutput []string
	for _, line := range windowLines {
		labelRendered := lipgloss.NewStyle().
			Foreground(line.color).
			Render(padRight(line.label, maxLabel))
		indent := strings.Repeat(" ", prefixWidth)
		wrapped := wrapLine(line.text, textWidth)
		for i, row := range wrapped {
			if i == 0 {
				logOutput = append(logOutput, labelRendered+" "+row)
			} else {
				logOutput = append(logOutput, indent+row)
			}
		}
	}
	if len(logOutput) > logHeight {
		logOutput = logOutput[len(logOutput)-logHeight:]
	}
	return logOutput
}
