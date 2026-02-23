package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Enter       key.Binding
	Back        key.Binding
	Refresh     key.Binding
	Search      key.Binding
	Delete      key.Binding
	RerunAll    key.Binding
	RerunFailed key.Binding
	Cancel      key.Binding
	ForceCancel key.Binding
	Info        key.Binding
	Filter       key.Binding
	ServerFilter key.Binding
	Up           key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
}

var Keys = KeyMap{
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
	ShiftTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-tab", "prev pane")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Delete:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	RerunAll:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun all")),
	RerunFailed: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "rerun failed")),
	Cancel:      key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "cancel run")),
	ForceCancel: key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "force cancel")),
	Info:        key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "run info")),
	Filter:       key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	ServerFilter: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "server filter")),
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/down", "down")),
	PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
	PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
}
