package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// allExitedMsg fires after all processes have exited and the 2-second delay has passed.
type allExitedMsg struct{}

type copyFlashDoneMsg struct{}
type copyConfirmDoneMsg struct{}

// SelectionState tracks the click-drag log row selection.
type SelectionState struct {
	active   bool
	startRow int // 0-based visual row index within the visible panel
	endRow   int
}

func (s SelectionState) minRow() int {
	if s.startRow <= s.endRow {
		return s.startRow
	}
	return s.endRow
}

func (s SelectionState) maxRow() int {
	if s.startRow >= s.endRow {
		return s.startRow
	}
	return s.endRow
}

// copyIconStr is the canonical display string for the copy-all icon.
// Used in both renderLogPanel and the mouse hit-detection in Update to keep
// the string and its computed width in sync.
const copyIconStr = "[⎘ copy all]"

var (
	accentColor  = lipgloss.Color("#7c3aed")
	runningColor = lipgloss.Color("#a3e635")
	exitedColor  = lipgloss.Color("#f87171")
	mutedColor   = lipgloss.Color("#555555")
	headerBg     = lipgloss.Color("#1a1a2e")
	panelBg      = lipgloss.Color("#0d0d1a")
	selectedBg   = lipgloss.NewStyle().Background(lipgloss.Color("#2d4a7a"))
)

type TabbedModel struct {
	pm               *ProcessManager
	keys             TabbedKeyMap
	selected         int
	scrollOffset     int // 0 = pinned to bottom (auto-scroll), >0 = scrolled up N lines
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

func NewTabbedModel(pm *ProcessManager, script string, delegated bool) TabbedModel {
	return TabbedModel{
		pm:        pm,
		keys:      DefaultTabbedKeyMap(),
		script:    script,
		delegated: delegated,
	}
}

func (m TabbedModel) Init() tea.Cmd {
	return nil
}

func (m TabbedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if m.interactive {
			if msg.String() == "ctrl+z" {
				m.interactive = false
				return m, nil
			}
			if !m.delegated && m.selected < len(m.pm.Processes) {
				if b := keyToBytes(msg.Key()); b != nil {
					_ = m.pm.Processes[m.selected].WriteStdin(b)
				}
			}
			return m, nil
		}
		switch {
		case msg.String() == "i":
			if !m.delegated && m.selected < len(m.pm.Processes) {
				m.interactive = true
			}
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.scrollOffset = 0 // reset scroll when switching tabs
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.pm.Processes)-1 {
				m.selected++
				m.scrollOffset = 0
			}
		case key.Matches(msg, m.keys.Restart):
			if !m.delegated && m.selected < len(m.pm.Processes) {
				m.allExitedPending = false
				go m.pm.Restart(m.pm.Processes[m.selected])
			}
		case key.Matches(msg, m.keys.RestartAll):
			m.allExitedPending = false
			go m.pm.RestartAll()
		case key.Matches(msg, m.keys.CopyKey):
			if m.sel.active {
				return m, tea.SetClipboard(m.selectedText())
			}
		case key.Matches(msg, m.keys.CopyAll):
			// No !m.delegated guard here — copying the full buffer is always safe
			// regardless of delegated mode. Contrast with Restart, which is blocked
			// in delegated mode to prevent unintended process restarts.
			m.copyFlash = true
			return m, tea.Batch(
				tea.SetClipboard(m.copyAllText()),
				tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
					return copyFlashDoneMsg{}
				}),
			)
		case msg.String() == "esc":
			m.sel = SelectionState{}
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if msg.Mod != 0 {
				// modifier held — let the terminal handle it (native select / open link)
				return m, nil
			}
			sw := m.sidebarWidth()
			listHeight := m.height - 2
			if listHeight < 0 {
				listHeight = 0
			}
			startIdx := m.sidebarStartIdx(listHeight)
			tabIdx := m.mouseToTabIdx(msg.X, msg.Y, sw, listHeight, startIdx, len(m.pm.Processes))
			if tabIdx >= 0 {
				m.selected = tabIdx
				m.scrollOffset = 0
				return m, nil
			}
			// Copy icon click: y==0, x in log panel, within icon hit area
			if msg.Y == 0 && msg.X > m.sidebarWidth() {
				iconW := lipgloss.Width(copyIconStr)
				logWidth := m.width - m.sidebarWidth() - 1
				// left column of the icon: log panel start + log panel width - icon width
				// log panel starts at sidebarWidth+1 (border col), so: sidebarWidth+1+logWidth-iconW
				copyIconX := m.sidebarWidth() + 1 + logWidth - iconW
				if msg.X >= copyIconX {
					m.copyFlash = true
					return m, tea.Batch(
						tea.SetClipboard(m.copyAllText()),
						tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
							return copyFlashDoneMsg{}
						}),
					)
				}
			}
			// existing log-row selection
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
			// Scroll log up (increase offset)
			if m.selected < len(m.pm.Processes) {
				maxScroll := m.pm.Processes[m.selected].Buffer.Len() - m.logHeight()
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.scrollOffset += 3
				if m.scrollOffset > maxScroll {
					m.scrollOffset = maxScroll
				}
			}
			return m, nil
		case tea.MouseWheelDown:
			// Scroll log down (decrease offset, 0 = bottom)
			m.scrollOffset -= 3
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		}

	case ProcessOutputMsg:
		// If pinned to bottom (offset 0), stay there. Otherwise hold position.
		return m, nil

	case ProcessStateMsg:
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
		return m, nil
	}

	return m, nil
}

func (m TabbedModel) copyAllText() string {
	if m.selected >= len(m.pm.Processes) {
		return ""
	}
	return strings.Join(m.pm.Processes[m.selected].Buffer.Lines(), "\n")
}

func (m TabbedModel) logHeight() int {
	h := m.height - 1 // minus log header
	if h < 0 {
		return 0
	}
	return h
}

func (m TabbedModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	sidebarWidth := m.sidebarWidth()
	logWidth := m.width - sidebarWidth - 1

	sidebar := m.renderSidebar(sidebarWidth, m.height)
	logs := m.renderLogPanel(logWidth, m.height)

	borderStr := strings.Repeat("│\n", m.height)
	if len(borderStr) > 0 {
		borderStr = borderStr[:len(borderStr)-1]
	}
	border := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Render(borderStr)

	v := tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, sidebar, border, logs))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m TabbedModel) sidebarWidth() int {
	w := m.width / 5
	if w < 16 {
		w = 16
	}
	if w > 40 {
		w = 40
	}
	return w
}

// sidebarStartIdx returns the first process index visible in the sidebar list
// given the available list height. Keeps m.selected visible.
func (m TabbedModel) sidebarStartIdx(listHeight int) int {
	if m.selected >= listHeight {
		return m.selected - listHeight + 1
	}
	return 0
}

func (m TabbedModel) renderSidebar(width, height int) string {
	if height < 1 {
		return ""
	}

	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(headerBg).
		Render(
			lipgloss.NewStyle().Foreground(accentColor).Background(headerBg).Bold(true).Render("miso") +
				lipgloss.NewStyle().Background(headerBg).Render(" ") +
				lipgloss.NewStyle().Foreground(mutedColor).Background(headerBg).Render(m.script),
		)

	divider := lipgloss.NewStyle().
		Width(width).
		Foreground(mutedColor).
		Background(headerBg).
		Render(strings.Repeat("─", width))

	// Available height for the workspace list (below header + divider)
	listHeight := height - 2
	if listHeight < 0 {
		listHeight = 0
	}

	// Scroll the tab list to keep selected visible
	procs := m.pm.Processes
	startIdx := m.sidebarStartIdx(listHeight)
	endIdx := startIdx + listHeight
	if endIdx > len(procs) {
		endIdx = len(procs)
	}

	var rows []string
	for i := startIdx; i < endIdx; i++ {
		proc := procs[i]
		label := proc.Entry.Label

		var statusText string
		statusFg := runningColor
		labelFg := lipgloss.Color("#aaaaaa")

		if proc.State == StateExited {
			if proc.ExitCode != 0 {
				statusText = fmt.Sprintf("✕ %d", proc.ExitCode)
				statusFg = exitedColor
			} else if !proc.StartedAt.IsZero() && time.Since(proc.StartedAt) < 5*time.Second {
				statusText = "■"
				statusFg = mutedColor
			} else {
				statusText = "✓"
				statusFg = runningColor
			}
		} else {
			statusText = "●"
		}

		// Content width = width minus 2 for Padding(0,1)
		// Label gets all space except status + 1 space gap
		statusW := lipgloss.Width(statusText)
		padW := width - 2 - statusW - 1
		if padW < 1 {
			padW = 1
		}

		if i == m.selected {
			bg := accentColor
			status := lipgloss.NewStyle().Foreground(statusFg).Background(bg).Render(statusText)
			labelRenderedSel := lipgloss.NewStyle().Foreground(labelFg).Background(bg).Render(padRight(label, padW))
			row := lipgloss.NewStyle().
				Width(width).
				Padding(0, 1).
				Background(bg).
				Render(labelRenderedSel + lipgloss.NewStyle().Background(bg).Render(" ") + status)
			rows = append(rows, row)
		} else {
			bg := headerBg
			status := lipgloss.NewStyle().Foreground(statusFg).Background(bg).Render(statusText)
			labelRendered := lipgloss.NewStyle().Foreground(labelFg).Background(bg).Render(padRight(label, padW))
			row := lipgloss.NewStyle().
				Width(width).
				Padding(0, 1).
				Background(bg).
				Render(labelRendered + lipgloss.NewStyle().Background(bg).Render(" ") + status)
			rows = append(rows, row)
		}
	}

	for len(rows) < listHeight {
		rows = append(rows, lipgloss.NewStyle().Width(width).Background(headerBg).Render(""))
	}

	content := header + "\n" + divider + "\n" + strings.Join(rows, "\n")

	return lipgloss.NewStyle().
		Width(width).
		MaxHeight(height).
		Background(headerBg).
		Render(content)
}

func (m TabbedModel) renderLogPanel(width, height int) string {
	if height < 1 || len(m.pm.Processes) == 0 {
		return ""
	}

	scrollHint := ""
	if m.scrollOffset > 0 {
		scrollHint = lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("(scrolled +%d)", m.scrollOffset))
	}

	var hintText string
	switch {
	case m.interactive:
		hintText = "interactive — ctrl+z to exit"
	case m.delegated:
		hintText = "c copy · R restart"
	default:
		hintText = "i interactive · c copy · r restart · R restart all"
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

	scrollHintLen := lipgloss.Width(scrollHint)
	hintsLen := lipgloss.Width(hints)
	// Use raw iconW for gap math, not lipgloss.Width(copyIcon) — ANSI codes inflate rendered width.
	// Width(width) with no padding: manually add the 1-col left pad in the content string.
	// Total content = 1 (left pad) + scrollHint + gap + hints + 1 (space) + icon + 1 (right pad)
	gap := width - 1 - scrollHintLen - hintsLen - 1 - iconW - 1 // 1+1+1 = left pad, space, right pad
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(width).
		Render(" " + scrollHint + strings.Repeat(" ", gap) + hints + " " + copyIcon + " ")

	// Log lines with scroll support
	logHeight := height - 1
	if logHeight < 0 {
		logHeight = 0
	}

	visualRows := m.buildLogVisualRows(width, logHeight)
	// Pad to fill remaining space.
	for len(visualRows) < logHeight {
		visualRows = append(visualRows, "")
	}

	// Highlight selected rows.
	for i := range visualRows {
		if m.sel.active && i >= m.sel.minRow() && i <= m.sel.maxRow() {
			visualRows[i] = selectedBg.Render(visualRows[i])
		}
	}

	logContent := strings.Join(visualRows, "\n")

	logPanel := lipgloss.NewStyle().
		Width(width).
		MaxHeight(logHeight).
		Padding(0, 1).
		Render(logContent)

	return header + "\n" + logPanel
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// mouseToTabIdx converts a mouse click to a process index in the sidebar.
// Returns -1 if the click is not on a valid process row.
// sidebarW, listHeight, startIdx, numProcs are passed in to keep the method pure/testable.
func (m TabbedModel) mouseToTabIdx(x, y, sidebarW, listHeight, startIdx, numProcs int) int {
	if x >= sidebarW {
		return -1 // not in sidebar
	}
	tabRow := y - 2 // skip header (y=0) and divider (y=1)
	if tabRow < 0 || tabRow >= listHeight {
		return -1
	}
	idx := startIdx + tabRow
	if idx >= numProcs {
		return -1
	}
	return idx
}

// mouseToLogRow converts absolute terminal coordinates to a 0-based visual
// log row index. Returns -1 if the coordinate is in the header, sidebar,
// or border column.
func (m TabbedModel) mouseToLogRow(x, y int) int {
	logPanelTop := 1 // row 0 is the log header
	if x <= m.sidebarWidth() {
		return -1 // in sidebar or border
	}
	row := y - logPanelTop
	if row < 0 {
		return -1
	}
	return row
}

// selectedText returns the selected log rows as a plain string.
func (m TabbedModel) selectedText() string {
	sidebarWidth := m.sidebarWidth()
	logWidth := m.width - sidebarWidth - 1
	logHeight := m.logHeight()
	visualRows := m.buildLogVisualRows(logWidth, logHeight)

	// Slice selection.
	lo := m.sel.minRow()
	hi := m.sel.maxRow()
	if lo < 0 {
		lo = 0
	}
	if hi >= len(visualRows) {
		hi = len(visualRows) - 1
	}
	if lo > hi || len(visualRows) == 0 {
		return ""
	}
	return strings.Join(visualRows[lo:hi+1], "\n")
}

// buildLogVisualRows re-derives the visual rows for the currently selected
// process's log panel. It applies the scroll window, wraps each raw line, and
// tail-slices to logHeight. logWidth is the panel width passed to renderLogPanel.
func (m TabbedModel) buildLogVisualRows(logWidth, logHeight int) []string {
	if m.selected >= len(m.pm.Processes) {
		return nil
	}
	proc := m.pm.Processes[m.selected]
	lines := proc.Buffer.Lines()
	totalLines := len(lines)

	endIdx := totalLines - m.scrollOffset
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - logHeight
	if startIdx < 0 {
		startIdx = 0
	}
	visible := lines[startIdx:endIdx]

	var visualRows []string
	for _, line := range visible {
		visualRows = append(visualRows, wrapLine(line, logWidth-2)...)
	}
	if len(visualRows) > logHeight {
		visualRows = visualRows[len(visualRows)-logHeight:]
	}
	return visualRows
}

// wrapLine splits a single log line into multiple visual rows of at most `width`
// printable columns. It is rune-aware. Always returns at least one row.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	runes := []rune(line)
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	var rows []string
	for len(runes) > 0 {
		// If remaining runes fit entirely, take them all.
		if lipgloss.Width(string(runes)) <= width {
			rows = append(rows, string(runes))
			break
		}
		// Binary-search for the largest rune-boundary prefix that fits.
		lo, hi := 0, len(runes)
		for lo+1 < hi {
			mid := (lo + hi) / 2
			if lipgloss.Width(string(runes[:mid])) <= width {
				lo = mid
			} else {
				hi = mid
			}
		}
		if lo == 0 {
			// Single rune is wider than width (e.g. full-width terminal art) — take it anyway.
			lo = 1
		}
		rows = append(rows, string(runes[:lo]))
		runes = runes[lo:]
	}
	return rows
}
