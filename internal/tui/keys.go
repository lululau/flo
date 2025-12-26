package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings for the application
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Enter    key.Binding

	// Actions
	Run           key.Binding
	Stop          key.Binding
	Refresh       key.Binding
	ToggleBookmark key.Binding
	FilterBookmark key.Binding
	FilterStatus  key.Binding
	SwitchToGroups key.Binding

	// Search
	Search     key.Binding
	SearchNext key.Binding
	SearchPrev key.Binding

	// External tools
	OpenEditor key.Binding
	OpenPager  key.Binding

	// Pagination
	NextPage key.Binding
	PrevPage key.Binding
	FirstPage key.Binding

	// Navigation/Exit
	Back key.Binding
	Quit key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u", "u"),
			key.WithHelp("pgup/u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d", "d"),
			key.WithHelp("pgdn/d", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "last"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),

		// Actions
		Run: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "run/refresh"),
		),
		Stop: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "stop"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ToggleBookmark: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "toggle bookmark"),
		),
		FilterBookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "filter bookmarks"),
		),
		FilterStatus: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "filter running"),
		),
		SwitchToGroups: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "groups"),
		),

		// Search
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		SearchNext: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		SearchPrev: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),

		// External tools
		OpenEditor: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "editor"),
		),
		OpenPager: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "pager"),
		),

		// Pagination
		NextPage: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev page"),
		),
		FirstPage: key.NewBinding(
			key.WithKeys("0"),
			key.WithHelp("0", "first page"),
		),

		// Navigation/Exit
		Back: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "quit"),
		),
	}
}

// PipelinesKeyMap returns key bindings specific to the pipelines page
type PipelinesKeyMap struct {
	KeyMap
}

// DefaultPipelinesKeyMap returns default pipelines page key bindings
func DefaultPipelinesKeyMap() PipelinesKeyMap {
	return PipelinesKeyMap{
		KeyMap: DefaultKeyMap(),
	}
}

// HistoryKeyMap returns key bindings specific to the history page
type HistoryKeyMap struct {
	KeyMap
}

// DefaultHistoryKeyMap returns default history page key bindings
func DefaultHistoryKeyMap() HistoryKeyMap {
	return HistoryKeyMap{
		KeyMap: DefaultKeyMap(),
	}
}

// LogsKeyMap returns key bindings specific to the logs page
type LogsKeyMap struct {
	KeyMap
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
}

// DefaultLogsKeyMap returns default logs page key bindings
func DefaultLogsKeyMap() LogsKeyMap {
	km := DefaultKeyMap()
	return LogsKeyMap{
		KeyMap: km,
		HalfPageUp: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "half page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "half page down"),
		),
	}
}

// ShortHelp returns a short help string for the key bindings
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Search, k.Back, k.Quit}
}

// FullHelp returns a full help string for the key bindings
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Enter, k.Run, k.Stop, k.Refresh},
		{k.Search, k.SearchNext, k.SearchPrev},
		{k.Back, k.Quit},
	}
}

