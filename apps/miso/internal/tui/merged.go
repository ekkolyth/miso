package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Fixed color palette for workspace labels — cycles if more workspaces than colors.
var labelColors = []lipgloss.Color{
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
	pm       *ProcessManager
	keys     MergedKeyMap
	cursor   int
	visible  map[int]bool
	logLines []mergedLine
	width    int
	height   int
	script   string
}

type mergedLine struct {
	label string
	color lipgloss.Color
	text  string
}

func NewMergedModel(pm *ProcessManager, script string) MergedModel {
	visible := make(map[int]bool)
	for i := range pm.Processes {
		visible[i] = true
	}

	return MergedModel{
		pm:      pm,
		keys:    DefaultMergedKeyMap(),
		visible: visible,
		script:  script,
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

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.pm.StopAll()
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
			if m.cursor < len(m.pm.Processes) {
				go m.pm.Restart(m.pm.Processes[m.cursor])
			}
		case key.Matches(msg, m.keys.RestartAll):
			go m.pm.RestartAll()
		}

	case ProcessOutputMsg:
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
		return m, nil
	}

	return m, nil
}

func (m MergedModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	filterBar := m.renderFilterBar()
	logHeight := m.height - 1 // minus filter bar

	// Build visible log lines
	var visibleLines []mergedLine
	visibleLabels := m.visibleLabels()
	for _, line := range m.logLines {
		if visibleLabels[line.label] {
			visibleLines = append(visibleLines, line)
		}
	}

	// Show last N lines
	if len(visibleLines) > logHeight {
		visibleLines = visibleLines[len(visibleLines)-logHeight:]
	}

	// Find max label width for padding
	maxLabel := 0
	for _, proc := range m.pm.Processes {
		if len(proc.Entry.Label) > maxLabel {
			maxLabel = len(proc.Entry.Label)
		}
	}

	var logOutput []string
	for _, line := range visibleLines {
		label := lipgloss.NewStyle().
			Foreground(line.color).
			Bold(true).
			Render(padRight(line.label, maxLabel))
		logOutput = append(logOutput, label+" "+line.text)
	}

	logContent := strings.Join(logOutput, "\n")
	lineCount := len(logOutput)
	if lineCount < logHeight {
		logContent += strings.Repeat("\n", logHeight-lineCount)
	}

	logPanel := lipgloss.NewStyle().
		Width(m.width).
		Height(logHeight).
		Padding(0, 1).
		Background(panelBg).
		Foreground(lipgloss.Color("#cccccc")).
		Render(logContent)

	return filterBar + "\n" + logPanel
}

func (m MergedModel) renderFilterBar() string {
	var items []string

	for i, proc := range m.pm.Processes {
		color := labelColors[i%len(labelColors)]
		label := proc.Entry.Label

		style := lipgloss.NewStyle().Padding(0, 1)

		if proc.State == StateExited && proc.ExitCode != 0 {
			color = exitedColor
		}

		if !m.visible[i] {
			style = style.
				Background(lipgloss.Color("#444444")).
				Foreground(lipgloss.Color("#888888")).
				Strikethrough(true)
		} else if i == m.cursor {
			style = style.
				Background(color).
				Foreground(lipgloss.Color("#ffffff")).
				Bold(true)
		} else {
			style = style.
				Background(color).
				Foreground(lipgloss.Color("#ffffff"))
		}

		items = append(items, style.Render(label))
	}

	labels := strings.Join(items, " ")
	hints := lipgloss.NewStyle().Foreground(mutedColor).Render("←→ select · space toggle · r restart · R restart all · q quit")

	gap := m.width - lipgloss.Width(labels) - lipgloss.Width(hints) - 4
	if gap < 1 {
		gap = 1
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Background(headerBg).
		Render(fmt.Sprintf("%s%s%s", labels, strings.Repeat(" ", gap), hints))
}

func (m MergedModel) colorForLabel(label string) lipgloss.Color {
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
