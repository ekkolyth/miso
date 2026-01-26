package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/ekkolyth/miso/internal/config"
	"github.com/ekkolyth/miso/internal/ui"
)

// return when user exits onboarding
var ErrAborted = errors.New("onboarding aborted")

// execute init wizard for new project and return config
func RunInitOnboarding(root string, managerNames []string, preselectedManager string, styles ui.Styles, logger *log.Logger) (config.Config, error) {
	if len(managerNames) == 0 {
		return config.Config{}, errors.New("no package managers are registered")
	}

	// Assume new project, ask for project name
	projectName, err := askProjectName(filepath.Base(root), styles)
	if err != nil {
		return config.Config{}, err
	}

	managerChoice, err := selectManager(managerNames, preselectedManager, styles)
	if err != nil {
		return config.Config{}, err
	}

	logger.Info("initialized project", "project", projectName, "manager", managerChoice)

	cfg := config.Config{
		PackageManager: managerChoice,
		ProjectName:    projectName,
		Scripts:        "./scripts",
		Flags:          make(map[string][]string),
	}
	return cfg, nil
}

func selectManager(options []string, preselected string, styles ui.Styles) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no package managers available to select")
	}

	choice := options[0]
	// use preselected if provided and valid
	if preselected != "" {
		for _, opt := range options {
			if opt == preselected {
				choice = preselected
				break
			}
		}
	}

	opts := make([]huh.Option[string], 0, len(options))
	for _, opt := range options {
		opts = append(opts, huh.NewOption(opt, opt))
	}

	selectField := huh.NewSelect[string]().
		Title(styles.Heading.Render("Pick your package manager")).
		Description(styles.Muted.Render("Press ↑/↓ to move, Enter to confirm • ctrl+c to bail")).
		Value(&choice).
		Options(opts...)

	form := huh.NewForm(
		huh.NewGroup(selectField),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return "", err
	}
	return choice, nil
}

func askProjectName(defaultName string, styles ui.Styles) (string, error) {
	model := newNameModel(defaultName, styles)
	final, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}
	if result, ok := final.(*nameModel); ok {
		if result.err != nil {
			return "", result.err
		}
		if result.value == "" {
			return "", errors.New("project name cannot be empty")
		}
		return result.value, nil
	}
	return "", errors.New("project name prompt failed")
}

type nameModel struct {
	input textinput.Model
	value string
	err   error
	style ui.Styles
}

func newNameModel(defaultName string, styles ui.Styles) *nameModel {
	input := textinput.New()
	input.Placeholder = defaultName
	input.Prompt = styles.Accent.Render("› ")
	input.CharLimit = 64
	input.SetValue(defaultName)
	input.Focus()

	return &nameModel{
		input: input,
		style: styles,
	}
}

func (m *nameModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *nameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.err = ErrAborted
			return m, tea.Quit
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				m.input.SetValue("")
				return m, nil
			}
			m.value = value
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *nameModel) View() string {
	help := m.style.Muted.Render("Enter a project name and press Enter.")
	return lipgloss.NewStyle().Padding(1, 2).Render(
		fmt.Sprintf("%s\n\n%s\n\n%s",
			m.style.Heading.Render("What is the name of the project?"),
			m.input.View(),
			help,
		),
	)
}
