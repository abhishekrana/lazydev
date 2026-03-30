package theme

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines all keybindings for the application.
type KeyMap struct {
	Quit        key.Binding
	TabNext     key.Binding
	TabPrev     key.Binding
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Enter       key.Binding
	Back        key.Binding
	Search      key.Binding
	Command     key.Binding
	Help        key.Binding
	Restart     key.Binding
	Stop        key.Binding
	Delete      key.Binding
	Describe    key.Binding
	Yaml        key.Binding
	Exec        key.Binding
	PortFwd     key.Binding
	Scale       key.Binding
	Filter      key.Binding
	ToggleFocus key.Binding
}

// Keys is the global keybinding set.
var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	TabNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	TabPrev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("S-tab", "prev tab"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/←", "sidebar"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/→", "logs"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "select/action"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Command: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "command"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Restart: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "restart"),
	),
	Stop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "stop"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Describe: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "describe"),
	),
	Yaml: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "yaml"),
	),
	Exec: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "exec"),
	),
	PortFwd: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "port-fwd"),
	),
	Scale: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "scale"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	ToggleFocus: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle focus"),
	),
}
