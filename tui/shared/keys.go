package shared

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up             key.Binding
	Down           key.Binding
	NextRepo       key.Binding
	PrevRepo       key.Binding
	Stage          key.Binding
	Unstage        key.Binding
	StageAll       key.Binding
	UnstageAll     key.Binding
	Diff           key.Binding
	Commit         key.Binding
	Open           key.Binding
	Help           key.Binding
	Quit           key.Binding
	Escape         key.Binding
	Branch         key.Binding
	ToggleGraph    key.Binding
	FocusDown      key.Binding
	FocusUp        key.Binding
	FocusLeft      key.Binding
	FocusRight     key.Binding
	GenerateMsg    key.Binding
	ContextSummary key.Binding
}

var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	NextRepo: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next repo"),
	),
	PrevRepo: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("S-tab", "prev repo"),
	),
	Stage: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "stage file"),
	),
	Unstage: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unstage file"),
	),
	StageAll: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "stage all"),
	),
	UnstageAll: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "unstage all"),
	),
	Diff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "view diff"),
	),
	Commit: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "commit"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open in nvim"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Branch: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "branches"),
	),
	ToggleGraph: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "toggle graph"),
	),
	FocusDown: key.NewBinding(
		key.WithKeys("ctrl+j"),
		key.WithHelp("C-j", "focus down"),
	),
	FocusUp: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("C-k", "focus up"),
	),
	FocusLeft: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("C-h", "focus left"),
	),
	FocusRight: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("C-l", "focus right"),
	),
	GenerateMsg: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "AI generate"),
	),
	ContextSummary: key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("C-x", "export context"),
	),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Stage, k.Diff, k.Commit, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.NextRepo, k.PrevRepo},
		{k.FocusLeft, k.FocusRight, k.FocusDown, k.FocusUp},
		{k.Stage, k.Unstage, k.StageAll, k.UnstageAll},
		{k.Diff, k.Commit, k.Open, k.Branch},
		{k.ToggleGraph, k.ContextSummary, k.Help, k.Quit, k.Escape},
	}
}
