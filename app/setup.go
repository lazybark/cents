package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SetupOptions drives the first-run screen that asks which database to use.
type SetupOptions struct {
	Note       string
	CreatePath string
	OpenPath   string
	// Choose is called when the user confirms a path. A returned error is
	// shown on the setup screen so the user can fix the path and retry.
	Choose func(path string, create bool) error
}

type setupStep int

const (
	setupStepChoice setupStep = iota
	setupStepPath
)

const (
	setupChoiceCreate = iota
	setupChoiceOpen
	setupChoiceCount
)

type setupModel struct {
	opts     SetupOptions
	step     setupStep
	cursor   int
	input    textinput.Model
	status   string
	chosen   bool
	quitting bool
	width    int
}

// RunSetup shows the database setup screen. It reports false when the user
// quit without choosing a database.
func RunSetup(opts SetupOptions) (bool, error) {
	input := textinput.New()
	input.Placeholder = "/path/to/cents.db"
	input.CharLimit = 1024
	input.Width = 60

	model := setupModel{
		opts:   opts,
		input:  input,
		status: "no database configured yet",
	}

	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return false, fmt.Errorf("setup failed: %w", err)
	}

	result, ok := final.(setupModel)

	return ok && result.chosen, nil
}

func (m setupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		if m.step == setupStepChoice {
			return m.updateChoice(msg)
		}

		return m.updatePath(msg)
	}

	return m, nil
}

func (m setupModel) updateChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = (m.cursor + setupChoiceCount - 1) % setupChoiceCount
	case "down", "j":
		m.cursor = (m.cursor + 1) % setupChoiceCount
	case "enter":
		m.step = setupStepPath
		m.input.SetValue(m.suggestedPath())
		m.input.CursorEnd()

		return m, m.input.Focus()
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m setupModel) updatePath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.step = setupStepChoice
		m.input.Blur()

		return m, nil
	case "enter":
		if err := m.opts.Choose(m.input.Value(), m.cursor == setupChoiceCreate); err != nil {
			m.status = err.Error()
			return m, nil
		}

		m.chosen = true
		m.quitting = true

		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m setupModel) suggestedPath() string {
	if m.cursor == setupChoiceCreate {
		return m.opts.CreatePath
	}

	return m.opts.OpenPath
}

func (m setupModel) View() string {
	if m.quitting {
		return ""
	}

	width := m.width
	if width == 0 {
		width = 100
	}

	contentWidth := width - 6
	if contentWidth < 76 {
		contentWidth = 76
	}

	lines := []string{sectionTitleStyle.Render("Choose a database"), ""}
	if m.opts.Note != "" {
		lines = append(lines, obligationStyle.Render(m.opts.Note), "")
	}

	lines = append(lines, mutedStyle.Render("Your choice is saved, so this screen only shows up again if the database goes missing."), "")

	labels := []string{"Create new database", "Open existing database"}
	for i, label := range labels {
		if i == m.cursor {
			lines = append(lines, selectedRowStyle.Render("> "+label))
		} else {
			lines = append(lines, rowStyle.Render("  "+label))
		}
	}

	lines = append(lines, "")
	if m.step == setupStepPath {
		lines = append(lines, fieldLabelStyle.Render("Database file"), inputBoxStyle.Render(m.input.View()), "")
		lines = append(lines, hintStyle.Render("Enter to confirm, Esc to go back, Ctrl+C to quit."))
	} else {
		lines = append(lines, hintStyle.Render("Use up/down to choose, Enter to continue, q to quit."))
	}

	body := panelStyle.Width(contentWidth).Render(strings.Join(lines, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, renderHeader(contentWidth), "", body, "", renderFooter(contentWidth, m.status))
}
