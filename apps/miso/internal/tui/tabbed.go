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
			m.pm.StopAll()
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
		// Trigger re-render
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

	border := lipgloss.NewStyle().
		Width(1).
		Height(m.height).
		Background(lipgloss.Color("#333333")).
		Render(strings.Repeat(" ", m.height))

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
	headerStyle := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(headerBg)

	header := headerStyle.Render(
		lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("miso") +
			" " +
			lipgloss.NewStyle().Foreground(mutedColor).Render(m.script),
	)

	var items []string
	for i, proc := range m.pm.Processes {
		label := proc.Entry.Label
		status := lipgloss.NewStyle().Foreground(runningColor).Render("●")
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#aaaaaa"))

		if proc.State == StateExited {
			if proc.ExitCode != 0 {
				status = lipgloss.NewStyle().Foreground(exitedColor).Render(fmt.Sprintf("✕ %d", proc.ExitCode))
				labelStyle = labelStyle.Foreground(exitedColor)
			} else {
				status = lipgloss.NewStyle().Foreground(mutedColor).Render("●")
			}
		}

		if i == m.selected {
			item := lipgloss.NewStyle().
				Width(width - 2).
				Padding(0, 1).
				Background(accentColor).
				Foreground(lipgloss.Color("#ffffff")).
				Render(padRight(label, width-8) + status)
			items = append(items, item)
		} else {
			item := lipgloss.NewStyle().
				Width(width - 2).
				Padding(0, 1).
				Render(labelStyle.Render(padRight(label, width-8)) + status)
			items = append(items, item)
		}
	}

	list := strings.Join(items, "\n")

	// Fill remaining height
	usedHeight := 1 + len(items) // header + items
	remaining := height - usedHeight
	if remaining < 0 {
		remaining = 0
	}
	padding := strings.Repeat("\n", remaining)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Background(headerBg).
		Render(header + "\n" + list + padding)
}

func (m TabbedModel) renderLogPanel(width, height int) string {
	if len(m.pm.Processes) == 0 {
		return ""
	}

	proc := m.pm.Processes[m.selected]

	// Header
	name := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(proc.Entry.Label)
	hints := lipgloss.NewStyle().Foreground(mutedColor).Render("↑↓ navigate · r restart · R restart all · q quit")

	headerWidth := width
	nameLen := lipgloss.Width(name)
	hintsLen := lipgloss.Width(hints)
	gap := headerWidth - nameLen - hintsLen - 2
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(headerBg).
		Render(name + strings.Repeat(" ", gap) + hints)

	// Log lines
	logHeight := height - 1 // minus header
	lines := proc.Buffer.Lines()

	// Show last N lines that fit
	if len(lines) > logHeight {
		lines = lines[len(lines)-logHeight:]
	}

	logContent := strings.Join(lines, "\n")

	// Fill remaining height
	lineCount := len(lines)
	if lineCount < logHeight {
		logContent += strings.Repeat("\n", logHeight-lineCount)
	}

	logPanel := lipgloss.NewStyle().
		Width(width).
		Height(logHeight).
		Padding(0, 1).
		Background(panelBg).
		Foreground(lipgloss.Color("#cccccc")).
		Render(logContent)

	return header + "\n" + logPanel
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
