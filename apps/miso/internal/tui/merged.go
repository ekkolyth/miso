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
	logSeq           int64
	scrollOffset     int // 0 = pinned to bottom, >0 = scrolled up N lines
	width            int
	height           int
	script           string
	delegated        bool
	allExitedPending bool
	sel              SelectionState
	interactive      bool
	copyFlash        bool // true during the 150ms invert flash
	copyConfirm      bool // true during the 1.5s green "✓ copied!" state
}

type mergedLine struct {
	label string
	color color.Color
	text  string
	seq   int64
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
		m.pm.ResizeAll(msg.Height, msg.Width)
		return m, nil

	case tea.PasteMsg:
		if m.interactive && !m.delegated && m.cursor < len(m.pm.Processes) {
			_ = m.pm.Processes[m.cursor].WriteStdin([]byte(msg.Content))
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.interactive {
			if msg.String() == "ctrl+z" {
				m.interactive = false
				return m, nil
			}
			if !m.delegated && m.cursor < len(m.pm.Processes) {
				if b := keyToBytes(msg.Key()); b != nil {
					_ = m.pm.Processes[m.cursor].WriteStdin(b)
				}
			}
			return m, nil
		}
		switch {
		case msg.String() == "i":
			if !m.delegated && m.cursor < len(m.pm.Processes) {
				m.interactive = true
			}
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
				m.sel = SelectionState{}
				go func() { _ = m.pm.Restart(m.pm.Processes[m.cursor]) }()
			}
		case key.Matches(msg, m.keys.RestartAll):
			m.allExitedPending = false
			m.sel = SelectionState{}
			go m.pm.RestartAll()
		case key.Matches(msg, m.keys.CopyKey):
			if m.sel.active {
				return m, tea.SetClipboard(m.selectedText())
			}
		case key.Matches(msg, m.keys.CopyAll):
			if !m.copyFlash && !m.copyConfirm {
				m.copyFlash = true
				return m, tea.Batch(
					tea.SetClipboard(m.copyAllText()),
					tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
						return copyFlashDoneMsg{}
					}),
				)
			}
		case msg.String() == "esc":
			m.sel = SelectionState{}
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if msg.Mod != 0 {
				// modifier held — let the terminal handle it (native select / open link)
				return m, nil
			}
			// Copy icon click: y==0, x within icon hit area at right of header
			if msg.Y == 0 {
				iconW := lipgloss.Width(copyIconStr)
				// Header has Padding(0,1), so content starts at col 1.
				// Icon is the last element: it ends at col (width-2) and starts at (width-2-iconW).
				copyIconX := m.width - 1 - iconW
				if msg.X >= copyIconX {
					if !m.copyFlash && !m.copyConfirm {
						m.copyFlash = true
						return m, tea.Batch(
							tea.SetClipboard(m.copyAllText()),
							tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
								return copyFlashDoneMsg{}
							}),
						)
					}
					return m, nil
				}
			}
			row := m.mouseToLogRow(msg.X, msg.Y)
			if row >= 0 {
				_, rowSeqs := m.buildLogVisualRows(m.logHeight(), SelectionState{})
				if row < len(rowSeqs) {
					seq := rowSeqs[row]
					m.sel = SelectionState{active: true, startSeq: seq, endSeq: seq}
				}
			}
		}

	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft && m.sel.active {
			row := m.mouseToLogRow(msg.X, msg.Y)
			if row >= 0 {
				_, rowSeqs := m.buildLogVisualRows(m.logHeight(), SelectionState{})
				if row < len(rowSeqs) {
					m.sel.endSeq = rowSeqs[row]
				}
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
		m.logSeq++
		m.logLines = append(m.logLines, mergedLine{
			label: msg.Label,
			color: color,
			text:  msg.Line,
			seq:   m.logSeq,
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

	case copyFlashDoneMsg:
		m.copyFlash = false
		m.copyConfirm = true
		return m, tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
			return copyConfirmDoneMsg{}
		})

	case copyConfirmDoneMsg:
		m.copyConfirm = false
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

	logOutput, _ := m.buildLogVisualRows(logHeight, m.sel)
	// Pad to fill.
	for len(logOutput) < logHeight {
		logOutput = append(logOutput, "")
	}

	// Note: selection highlighting is applied inside buildLogVisualRows, where
	// the label and text are still separate pieces. Applying selectedBg.Render()
	// here (after assembly) doesn't work because the label's trailing ANSI reset
	// cancels the background before the log text is painted.

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
	switch {
	case m.interactive:
		hintText = "interactive — ctrl+z to exit"
	case m.delegated:
		hintText = "space toggle · c copy · R restart"
	default:
		hintText = "i interactive · space toggle · c copy · r restart · R restart all"
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(hintText)

	iconW := lipgloss.Width(copyIconStr)
	var copyIcon string
	switch {
	case m.copyFlash:
		copyIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#111111")).
			Background(lipgloss.Color("#aaaaaa")).
			Render(copyIconStr)
	case m.copyConfirm:
		copyIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e")).
			Render("[✓ copied! ]")
	default:
		copyIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Render(copyIconStr)
	}

	titleLen := lipgloss.Width(title)
	scrollHintLen := lipgloss.Width(scrollHint)
	hintsLen := lipgloss.Width(hints)
	// Layout: title | gap | scrollHint hints [space] copyIcon [space]
	// Padding(0,1) accounts for 1 col on each side.
	gap := m.width - 2 - titleLen - scrollHintLen - hintsLen - 1 - iconW - 1
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(title + strings.Repeat(" ", gap) + scrollHint + hints + " " + copyIcon)

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

// copyAllText returns the raw log text for all currently visible processes,
// interleaved in arrival order. If a process is toggled off it is excluded.
func (m MergedModel) copyAllText() string {
	visibleLabels := m.visibleLabels()
	var lines []string
	for _, line := range m.logLines {
		if visibleLabels[line.label] {
			lines = append(lines, line.text)
		}
	}
	return strings.Join(lines, "\n")
}

// mouseToLogRow converts absolute terminal coordinates to a 0-based visual
// log row index. Returns -1 if the coordinate is above the log area.
func (m MergedModel) mouseToLogRow(_, y int) int {
	logPanelTop := 3 // row 0=header, 1=tabs, 2=selector, 3+=logs
	row := y - logPanelTop
	if row < 0 {
		return -1
	}
	return row
}

// selected visible-process log lines as raw text
func (m MergedModel) selectedText() string {
	if !m.sel.active {
		return ""
	}
	visibleLabels := m.visibleLabels()
	lo, hi := m.sel.minSeq(), m.sel.maxSeq()

	var out []string
	for _, line := range m.logLines {
		if visibleLabels[line.label] && line.seq >= lo && line.seq <= hi {
			out = append(out, line.text)
		}
	}
	return strings.Join(out, "\n")
}

// buildLogVisualRows re-derives the visual rows for the merged log panel,
// applying the scroll window, wrapping, and tail-slice. The second return
// value is the source line's seq for each visual row, parallel to the first.
// sel is used to apply selection highlighting row-by-row; pass SelectionState{}
// (inactive) to skip highlighting (e.g. when building rows for clipboard copy).
func (m MergedModel) buildLogVisualRows(logHeight int, sel SelectionState) ([]string, []int64) {
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
	var seqs []int64
	for _, line := range windowLines {
		indent := strings.Repeat(" ", prefixWidth)
		wrapped := wrapLine(line.text, textWidth)
		selected := sel.active && line.seq >= sel.minSeq() && line.seq <= sel.maxSeq()
		for i, row := range wrapped {
			if i == 0 {
				// Render label and text under the same background so the selection
				// highlight covers the full line. Applying selectedBg to the
				// pre-rendered label string doesn't work because the label's
				// trailing ANSI reset (\x1b[0m) cancels the background mid-line.
				if selected {
					labelPart := lipgloss.NewStyle().
						Foreground(line.color).
						Background(selectedBg.GetBackground()).
						Render(padRight(line.label, maxLabel))
					textPart := selectedBg.Render(" " + row)
					logOutput = append(logOutput, labelPart+textPart)
				} else {
					labelRendered := lipgloss.NewStyle().
						Foreground(line.color).
						Render(padRight(line.label, maxLabel))
					logOutput = append(logOutput, labelRendered+" "+row)
				}
			} else {
				if selected {
					logOutput = append(logOutput, selectedBg.Render(indent+row))
				} else {
					logOutput = append(logOutput, indent+row)
				}
			}
			seqs = append(seqs, line.seq)
		}
	}
	if len(logOutput) > logHeight {
		logOutput = logOutput[len(logOutput)-logHeight:]
		seqs = seqs[len(seqs)-logHeight:]
	}
	return logOutput, seqs
}
