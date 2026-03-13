package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// allExitedMsg fires after all processes have exited and the 2-second delay has passed.
type allExitedMsg struct{}

var (
	accentColor  = lipgloss.Color("#7c3aed")
	runningColor = lipgloss.Color("#a3e635")
	exitedColor  = lipgloss.Color("#f87171")
	mutedColor   = lipgloss.Color("#555555")
	headerBg     = lipgloss.Color("#1a1a2e")
	panelBg      = lipgloss.Color("#0d0d1a")
)

type TabbedModel struct {
	pm              *ProcessManager
	keys            TabbedKeyMap
	selected        int
	scrollOffset    int // 0 = pinned to bottom (auto-scroll), >0 = scrolled up N lines
	width           int
	height          int
	script          string
	delegated       bool
	allExitedPending bool
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

	case tea.KeyMsg:
		switch {
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
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
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
		case tea.MouseButtonWheelDown:
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
	}

	return m, nil
}

func (m TabbedModel) logHeight() int {
	h := m.height - 1 // minus log header
	if h < 0 {
		return 0
	}
	return h
}

func (m TabbedModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
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

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, border, logs)
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
	startIdx := 0
	if m.selected >= listHeight {
		startIdx = m.selected - listHeight + 1
	}
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
				labelFg = exitedColor
			} else {
				statusText = "✓"
				statusFg = runningColor
			}
		} else {
			statusText = "●"
		}

		padW := width - 5
		if padW < 1 {
			padW = 1
		}

		if i == m.selected {
			bg := accentColor
			status := lipgloss.NewStyle().Foreground(statusFg).Background(bg).Render(statusText)
			row := lipgloss.NewStyle().
				Width(width).
				Padding(0, 1).
				Background(bg).
				Foreground(lipgloss.Color("#ffffff")).
				Render(padRight(label, padW) + status)
			rows = append(rows, row)
		} else {
			bg := headerBg
			status := lipgloss.NewStyle().Foreground(statusFg).Background(bg).Render(statusText)
			labelRendered := lipgloss.NewStyle().Foreground(labelFg).Background(bg).Render(padRight(label, padW))
			row := lipgloss.NewStyle().
				Width(width).
				Padding(0, 1).
				Background(bg).
				Render(labelRendered + status)
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

	proc := m.pm.Processes[m.selected]

	// Header (1 line)
	name := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(proc.Entry.Label)

	scrollHint := ""
	if m.scrollOffset > 0 {
		scrollHint = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Render(fmt.Sprintf(" (scrolled +%d)", m.scrollOffset))
	}

	hintText := "↑↓ navigate · r restart · R restart all · ctrl+c quit"
	if m.delegated {
		hintText = "↑↓ navigate · R restart · ctrl+c quit"
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(hintText)

	nameLen := lipgloss.Width(name) + lipgloss.Width(scrollHint)
	hintsLen := lipgloss.Width(hints)
	gap := width - nameLen - hintsLen - 2
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Render(name + scrollHint + strings.Repeat(" ", gap) + hints)

	// Log lines with scroll support
	logHeight := height - 1
	if logHeight < 0 {
		logHeight = 0
	}

	lines := proc.Buffer.Lines()
	totalLines := len(lines)

	// Calculate visible window based on scroll offset
	endIdx := totalLines - m.scrollOffset
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - logHeight
	if startIdx < 0 {
		startIdx = 0
	}

	visible := lines[startIdx:endIdx]

	// Build exactly logHeight rows
	var logRows []string
	for _, line := range visible {
		if lipgloss.Width(line) > width-2 {
			line = line[:width-2]
		}
		logRows = append(logRows, line)
	}
	for len(logRows) < logHeight {
		logRows = append(logRows, "")
	}

	logContent := strings.Join(logRows, "\n")

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
