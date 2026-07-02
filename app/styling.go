package app

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

var (
	appTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4E9D8"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#9C927F"))
	headlineStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#D0CABD"))
	statusLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#D2B574")).Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D")).Underline(true)
	tableHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#DCC48A"))
	hintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#B5AC9D")).Italic(true)
	badgeStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#C9A86A")).Bold(true).Padding(0, 1)
	modeBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#5F4C2F")).Padding(0, 1)
	progressFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C"))
	progressRestStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F5A52"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	buttonStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#4E4334")).Padding(0, 1)
	buttonActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D")).Bold(true).Padding(0, 1)
	fieldLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7C96D")).Bold(true)
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D"))
	rowStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8"))
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	obligationStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	positiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C")).Bold(true)
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Add      key.Binding
	List     key.Binding
	Help     key.Binding
	MoveLeft key.Binding
	MoveRght key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "prev group"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next group"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "prev item"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "next item"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add account"),
		),
		List: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "list accounts"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Add, k.List, k.Quit},
	}
}
