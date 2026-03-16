package tui

import "github.com/charmbracelet/bubbles/key"

type TabbedKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Restart    key.Binding
	RestartAll key.Binding
	Select     key.Binding
	Quit       key.Binding
}

func DefaultTabbedKeyMap() TabbedKeyMap {
	return TabbedKeyMap{
		Up:         key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:       key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Restart:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
		RestartAll: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart all")),
		Select:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "select")),
		Quit:       key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

type MergedKeyMap struct {
	Left       key.Binding
	Right      key.Binding
	Toggle     key.Binding
	Restart    key.Binding
	RestartAll key.Binding
	Select     key.Binding
	Quit       key.Binding
}

func DefaultMergedKeyMap() MergedKeyMap {
	return MergedKeyMap{
		Left:       key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "left")),
		Right:      key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "right")),
		Toggle:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Restart:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
		RestartAll: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart all")),
		Select:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "select")),
		Quit:       key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}
