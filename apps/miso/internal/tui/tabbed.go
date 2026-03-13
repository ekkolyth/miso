package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	accentColor  = lipgloss.Color("#7c3aed")
	runningColor = lipgloss.Color("#a3e635")
	exitedColor  = lipgloss.Color("#f87171")
	mutedColor   = lipgloss.Color("#555555")
	headerBg     = lipgloss.Color("#1a1a2e")
	panelBg      = lipgloss.Color("#0d0d1a")
)

type TabbedModel struct {
	pm       *ProcessManager
	keys     TabbedKeyMap
	selected int
	width    int
	height   int
	script   string
}

func NewTabbedModel(pm *ProcessManager, script string) TabbedModel {
	return TabbedModel{
		pm:     pm,
		keys:   DefaultTabbedKeyMap(),
		script: script,
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
			// Don't block here — StopAll runs after p.Run() returns in launch.go
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.pm.Processes)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Restart):
			if m.selected < len(m.pm.Processes) {
				go m.pm.Restart(m.pm.Processes[m.selected])
			}
		case key.Matches(msg, m.keys.RestartAll):
			go m.pm.RestartAll()
		}

	case ProcessOutputMsg, ProcessStateMsg:
		return m, nil
	}

	return m, nil
}

func (m TabbedModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	sidebarWidth := m.sidebarWidth()
	logWidth := m.width - sidebarWidth - 1 // -1 for border

	sidebar := m.renderSidebar(sidebarWidth, m.height)
	logs := m.renderLogPanel(logWidth, m.height)

	// Single-character vertical border clamped to height
	borderStr := strings.Repeat("│\n", m.height)
	if len(borderStr) > 0 {
		borderStr = borderStr[:len(borderStr)-1] // trim trailing newline
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
	if w > 30 {
		w = 30
	}
	return w
}

func (m TabbedModel) renderSidebar(width, height int) string {
	if height < 1 {
		return ""
	}

	// Header takes 1 line
	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(headerBg).
		Render(
			lipgloss.NewStyle().Foreground(accentColor).Background(headerBg).Bold(true).Render("miso") +
				" " +
				lipgloss.NewStyle().Foreground(mutedColor).Background(headerBg).Render(m.script),
		)

	// Available height for the workspace list (below header)
	listHeight := height - 1
	if listHeight < 0 {
		listHeight = 0
	}

	// Build visible items — clamp to listHeight and scroll to keep selected visible
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
				statusText = "●"
				statusFg = mutedColor
			}
		} else {
			statusText = "●"
		}

		padW := width - 8
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

	// Pad remaining lines so sidebar is always exactly `height` lines
	for len(rows) < listHeight {
		rows = append(rows, lipgloss.NewStyle().Width(width).Background(headerBg).Render(""))
	}

	content := header + "\n" + strings.Join(rows, "\n")

	return lipgloss.NewStyle().
		Width(width).
		MaxHeight(height).
		Render(content)
}

func (m TabbedModel) renderLogPanel(width, height int) string {
	if height < 1 || len(m.pm.Processes) == 0 {
		return ""
	}

	proc := m.pm.Processes[m.selected]

	// Header (1 line)
	name := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(proc.Entry.Label)
	hints := lipgloss.NewStyle().Foreground(mutedColor).Render("↑↓ navigate · r restart · R restart all · ctrl+c quit")

	nameLen := lipgloss.Width(name)
	hintsLen := lipgloss.Width(hints)
	gap := width - nameLen - hintsLen - 2
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Render(name + strings.Repeat(" ", gap) + hints)

	// Log lines — strictly clamped to available height
	logHeight := height - 1
	if logHeight < 0 {
		logHeight = 0
	}

	lines := proc.Buffer.Lines()

	// Show only the last logHeight lines (auto-scroll)
	if len(lines) > logHeight {
		lines = lines[len(lines)-logHeight:]
	}

	// Build exactly logHeight rows
	var logRows []string
	for _, line := range lines {
		// Truncate long lines to prevent wrapping
		if lipgloss.Width(line) > width-2 {
			line = line[:width-2]
		}
		logRows = append(logRows, line)
	}
	// Pad with empty lines to fill the panel
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
