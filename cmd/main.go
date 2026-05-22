package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ledgerEntry struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	Kind        string
	AmountCents int64
	Note        string
}

type model struct {
	db        *gorm.DB
	dbPath    string
	created   bool
	entries   []ledgerEntry
	input     textinput.Model
	status    string
	width     int
	height    int
	quitting  bool
	hasLoaded bool
}

var (
	appTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4E9D8"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9C927F"))
	panelStyle    = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6F5F47")).
		Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#C8C1B2"))
	entryKindIncome    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DB88A")).Bold(true)
	entryKindExpense   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D27D7D")).Bold(true)
	inputStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
)

func main() {
	db, dbPath, created, err := openDatabase()
	if err != nil {
		fmt.Fprintln(os.Stderr, "database init failed:", err)
		os.Exit(1)
	}

	entries, err := loadEntries(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database read failed:", err)
		os.Exit(1)
	}

	m := newModel(db, dbPath, created, entries)
	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "program failed:", err)
		os.Exit(1)
	}
}

func openDatabase() (*gorm.DB, string, bool, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, "", false, err
	}

	dbPath := filepath.Join(workingDir, "cents.db")
	_, statErr := os.Stat(dbPath)
	created := os.IsNotExist(statErr)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, "", false, statErr
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, "", false, err
	}

	if err := db.AutoMigrate(&ledgerEntry{}); err != nil {
		return nil, "", false, err
	}

	return db, dbPath, created, nil
}

func loadEntries(db *gorm.DB) ([]ledgerEntry, error) {
	var entries []ledgerEntry
	if err := db.Order("created_at desc, id desc").Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

func newModel(db *gorm.DB, dbPath string, created bool, entries []ledgerEntry) model {
	input := textinput.New()
	input.Placeholder = "income 2500 salary"
	input.Focus()
	input.CharLimit = 120
	input.Width = 44

	status := databaseStatus(created, len(entries), dbPath)
	if len(entries) == 0 {
		status = status + " | no ledger entries yet"
	}

	return model{
		db:      db,
		dbPath:  dbPath,
		created: created,
		entries: entries,
		input:   input,
		status:  status,
	}
}

func databaseStatus(created bool, count int, dbPath string) string {
	if created {
		return "created " + dbPath + " and loaded " + strconv.Itoa(count) + " entries"
	}

	return "opened " + dbPath + " with " + strconv.Itoa(count) + " entries"
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			return m.submitEntry()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	width := m.width
	if width == 0 {
		width = 80
	}

	headline := appTitleStyle.Render("CENTS") + "\n" + mutedStyle.Render("Simple personal finance tracker")
	header := panelStyle.Width(clamp(width-4, 34, width)).Render(headline)

	entriesPanel := m.renderEntriesPanel(clamp(width-4, 34, width))
	inputPanel := m.renderInputPanel(clamp(width-4, 34, width))
	footer := statusStyle.Render(m.status)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", entriesPanel, "", inputPanel, "", footer)
}

func (m model) submitEntry() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	if strings.EqualFold(value, "help") {
		m.status = "use: income <amount> <note> | expense <amount> <note> | q to quit"
		m.input.SetValue("")
		return m, nil
	}

	if strings.EqualFold(value, "clear") {
		if err := m.db.Delete(&ledgerEntry{}).Error; err != nil {
			m.status = "clear failed: " + err.Error()
			return m, nil
		}

		m.entries = nil
		m.status = "ledger cleared"
		m.input.SetValue("")
		return m, nil
	}

	entry, err := parseEntry(value)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.entries = append([]ledgerEntry{entry}, m.entries...)
	m.status = fmt.Sprintf("saved %s for %s", formatMoney(entry.AmountCents), entry.Note)
	m.input.SetValue("")
	return m, nil
}

func (m model) renderEntriesPanel(width int) string {
	lines := make([]string, 0, len(m.entries)+2)
	lines = append(lines, sectionTitleStyle.Render("Ledger"))
	if len(m.entries) == 0 {
		lines = append(lines, mutedStyle.Render("No entries yet. Start with: income 2500 salary"))
	} else {
		for _, entry := range m.entries {
			kindStyle := entryKindExpense
			if entry.Kind == "income" {
				kindStyle = entryKindIncome
			}

			lines = append(lines, fmt.Sprintf("%s  %s  %s", kindStyle.Render(strings.ToUpper(entry.Kind)), formatMoney(entry.AmountCents), mutedStyle.Render(entry.Note)))
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderInputPanel(width int) string {
	inputLine := inputStyle.Width(width - 4).Render(m.input.View())
	helpText := mutedStyle.Render("Commands: income <amount> <note>, expense <amount> <note>, clear, help, q")

	return panelStyle.Width(width).Render(sectionTitleStyle.Render("Quick add") + "\n" + inputLine + "\n" + helpText)
}

func parseEntry(value string) (ledgerEntry, error) {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return ledgerEntry{}, errors.New("use: income <amount> <note> or expense <amount> <note>")
	}

	kind := strings.ToLower(parts[0])
	if kind != "income" && kind != "expense" {
		return ledgerEntry{}, errors.New("first word must be income or expense")
	}

	amount, err := parseAmountCents(parts[1])
	if err != nil {
		return ledgerEntry{}, err
	}

	note := strings.TrimSpace(strings.TrimPrefix(value, parts[0]))
	note = strings.TrimSpace(strings.TrimPrefix(note, parts[1]))
	if note == "" {
		return ledgerEntry{}, errors.New("note is required")
	}

	return ledgerEntry{Kind: kind, AmountCents: amount, Note: note}, nil
}

func parseAmountCents(raw string) (int64, error) {
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New("amount must be a number")
	}

	if amount <= 0 {
		return 0, errors.New("amount must be greater than zero")
	}

	return int64(math.Round(amount * 100)), nil
}

func formatMoney(cents int64) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100)
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
