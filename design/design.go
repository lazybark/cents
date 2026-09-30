// Package main is a standalone, context-free extraction of this project's
// TUI design system: the same colors, panels, buttons, tables and hotkey
// workflow, wired up around generic placeholder content (groups, items,
// entries) instead of any domain-specific concepts. Copy this single file
// into a new module, `go get` the three dependencies below, and reshape the
// generic screens (menu, add form, list table, edit form, settings, export,
// overview) to fit a new app while keeping the same look and feel.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	program := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Design tokens: the same palette, panels and buttons as the source app.
// ---------------------------------------------------------------------------

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
	progressFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C"))
	progressRestStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F5A52"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	buttonStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#4E4334")).Padding(0, 1)
	buttonActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D")).Bold(true).Padding(0, 1)
	fieldLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7C96D")).Bold(true)
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D"))
	rowStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8"))
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	negativeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	positiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C")).Bold(true)
)

// ---------------------------------------------------------------------------
// Hotkeys.
// ---------------------------------------------------------------------------

type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
	Add   key.Binding
	List  key.Binding
	Help  key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "prev group")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next group")),
		Left:  key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "prev item")),
		Right: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next item")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Add:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add item")),
		List:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "list items")),
		Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
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

// ---------------------------------------------------------------------------
// Screens and menu structure.
// ---------------------------------------------------------------------------

type screen int

const (
	screenMenu screen = iota
	screenAddForm
	screenListTable
	screenEditForm
	screenSettings
	screenExport
	screenOverview
)

type menuGroup struct {
	title string
	items []string
}

// appMenuGroups mirrors the source app's shape: eight groups laid out in two
// columns, most of them following a "new / mode a / mode b / ..." pattern,
// one pair reachable via the a/e shortcuts, and a trailing settings group.
func appMenuGroups() []menuGroup {
	return []menuGroup{
		{title: "Group 1", items: []string{"New Type A", "New Type B", "History", "Overview"}},
		{title: "Group 2", items: []string{"add item", "list items"}},
		{title: "Group 3", items: []string{"new", "mode a", "mode b"}},
		{title: "Group 4", items: []string{"new", "mode a", "mode b", "mode c"}},
		{title: "Group 5", items: []string{"new", "mode a", "mode b", "mode c"}},
		{title: "Group 6", items: []string{"new", "mode a", "mode b"}},
		{title: "Group 7", items: []string{"new", "mode a", "mode b"}},
		{title: "Group 8", items: []string{"settings", "export"}},
	}
}

// ---------------------------------------------------------------------------
// Generic domain: one entity type ("item") stands in for whatever real
// records a concrete app would manage.
// ---------------------------------------------------------------------------

type item struct {
	id          int
	name        string
	description string
	category    string
	mode        int
	amountCents int64
	ignore      bool
	updatedAt   time.Time
}

type historyEntry struct {
	day        time.Time
	valueCents int64
}

func sampleCategoryOptions() []string {
	return []string{"Category A", "Category B", "Category C", "Category D"}
}

func modeLabel(mode int) string {
	labels := []string{"Mode A", "Mode B", "Mode C"}
	if mode < 0 || mode >= len(labels) {
		return "Mode A"
	}
	return labels[mode]
}

func seedItems() []item {
	now := time.Now()
	return []item{
		{id: 1, name: "Entry One", description: "First sample record", category: "Category A", mode: 0, amountCents: 125000, updatedAt: now.AddDate(0, 0, -2)},
		{id: 2, name: "Entry Two", description: "Second sample record", category: "Category B", mode: 1, amountCents: 45000, updatedAt: now.AddDate(0, 0, -6)},
		{id: 3, name: "Entry Three", description: "Third sample record", category: "Category A", mode: 0, amountCents: 89000, updatedAt: now.AddDate(0, -1, 0)},
		{id: 4, name: "Entry Four", description: "Fourth sample record", category: "Category C", mode: 2, amountCents: 300000, ignore: true, updatedAt: now.AddDate(0, -2, -3)},
		{id: 5, name: "Entry Five", description: "Fifth sample record", category: "Category D", mode: 1, amountCents: 15075, updatedAt: now.AddDate(0, 0, -1)},
		{id: 6, name: "Entry Six", description: "Sixth sample record", category: "Category B", mode: 2, amountCents: 62000, ignore: true, updatedAt: now.AddDate(0, -1, -10)},
	}
}

// ---------------------------------------------------------------------------
// Add-item form: name / description / category select / amount / checkbox,
// with a shared "Active" cursor spanning all five positions.
// ---------------------------------------------------------------------------

const addFormFieldCount = 5

const (
	addFieldName = iota
	addFieldDescription
	addFieldCategory
	addFieldAmount
	addFieldIgnore
)

type addItemForm struct {
	NameInput        textinput.Model
	DescriptionInput textinput.Model
	AmountInput      textinput.Model
	CategoryOptions  []string
	CategoryIndex    int
	Ignore           bool
	Active           int
}

func newAddItemForm(categoryOptions []string) addItemForm {
	name := textinput.New()
	name.Placeholder = "Entry name"
	name.CharLimit = 80
	name.Width = 34

	description := textinput.New()
	description.Placeholder = "Short description"
	description.CharLimit = 80
	description.Width = 34

	amount := textinput.New()
	amount.Placeholder = "1234.56"
	amount.CharLimit = 24
	amount.Width = 20

	form := addItemForm{
		NameInput:        name,
		DescriptionInput: description,
		AmountInput:      amount,
		CategoryOptions:  append([]string(nil), categoryOptions...),
	}

	return form.focusActive()
}

func (f addItemForm) focusActive() addItemForm {
	f.NameInput.Blur()
	f.DescriptionInput.Blur()
	f.AmountInput.Blur()

	switch f.Active {
	case addFieldName:
		f.NameInput.Focus()
	case addFieldDescription:
		f.DescriptionInput.Focus()
	case addFieldAmount:
		f.AmountInput.Focus()
	}

	return f
}

func (f addItemForm) next() addItemForm {
	if f.Active < addFormFieldCount-1 {
		f.Active++
	}

	return f.focusActive()
}

func (f addItemForm) prev() addItemForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.focusActive()
}

// ---------------------------------------------------------------------------
// Application model.
// ---------------------------------------------------------------------------

const (
	editFieldValue = iota
	editFieldUpdateLog
	editFieldIgnore
	editFieldLogDate
	editFieldLogValue
	editFieldCount
)

type settingsEditMode int

const (
	settingsEditNone settingsEditMode = iota
	settingsEditOption
	settingsEditCategory
)

type model struct {
	items   []item
	history map[int][]historyEntry
	nextID  int

	settingsOption     string
	settingsCategories []string

	screen    screen
	menuGroup int
	menuItem  int

	listFilterMode int // -1 = show everything, -2 = "history" (ignored) view, 0..2 = mode filter
	listTitle      string
	cursor         int
	sortField      int
	sortMenuOpen   bool
	sortCursor     int

	deleteConfirmActive bool
	deleteConfirmID     int
	deleteConfirmName   string

	addForm addItemForm

	editID            int
	editActiveField   int
	editValueInput    textinput.Model
	editUpdateLog     bool
	editIgnore        bool
	editLogDateInput  textinput.Model
	editLogValueInput textinput.Model

	settingsCursor    int
	settingsEditMode  settingsEditMode
	settingsEditIndex int
	settingsEditInput textinput.Model

	exportDatasetIndex int
	exportFormatIndex  int
	exportPathInput    textinput.Model
	exportActive       int

	status   string
	width    int
	height   int
	quitting bool

	help help.Model
	keys keyMap
}

func newModel() model {
	items := seedItems()

	editValue := textinput.New()
	editValue.Placeholder = "1234.56"
	editValue.CharLimit = 24
	editValue.Width = 20

	editLogDate := textinput.New()
	editLogDate.Placeholder = "DD.MM.YYYY"
	editLogDate.CharLimit = 24
	editLogDate.Width = 20

	editLogValue := textinput.New()
	editLogValue.Placeholder = "1234.56"
	editLogValue.CharLimit = 24
	editLogValue.Width = 20

	settingsInput := textinput.New()
	settingsInput.Placeholder = "value"
	settingsInput.CharLimit = 40
	settingsInput.Width = 28

	exportPath := textinput.New()
	exportPath.Placeholder = "/path/to/output"
	exportPath.CharLimit = 120
	exportPath.Width = 34
	exportPath.SetValue(defaultExportPath())

	helpModel := help.New()
	helpModel.ShowAll = false

	return model{
		items:              items,
		history:            map[int][]historyEntry{},
		nextID:             len(items) + 1,
		settingsOption:     "value one",
		settingsCategories: []string{"Category 1", "Category 2", "Category 3"},
		screen:             screenMenu,
		listFilterMode:     -1,
		addForm:            newAddItemForm(sampleCategoryOptions()),
		editValueInput:     editValue,
		editLogDateInput:   editLogDate,
		editLogValueInput:  editLogValue,
		settingsEditInput:  settingsInput,
		exportPathInput:    exportPath,
		status:             fmt.Sprintf("loaded %d sample items", len(items)),
		help:               helpModel,
		keys:               newKeyMap(),
	}
}

func defaultExportPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return workingDir
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.help, cmd = m.help.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width - 6
		if m.help.Width < 60 {
			m.help.Width = 60
		}

		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true

			return m, tea.Quit
		}

		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenAddForm:
			return m.updateAddForm(msg)
		case screenListTable:
			return m.updateListTable(msg)
		case screenEditForm:
			return m.updateEditForm(msg)
		case screenSettings:
			return m.updateSettings(msg)
		case screenExport:
			return m.updateExport(msg)
		case screenOverview:
			return m.updateOverview(msg)
		}
	}

	return m, cmd
}

func (m model) View() string {
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

	header := renderHeader(contentWidth)
	body := m.renderBody(contentWidth)
	footer := renderFooter(contentWidth, m.status)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
}

func (m model) renderBody(width int) string {
	switch m.screen {
	case screenAddForm:
		return m.renderAddForm(width)
	case screenListTable:
		return m.renderListTable(width)
	case screenEditForm:
		return m.renderEditForm(width)
	case screenSettings:
		return m.renderSettings(width)
	case screenExport:
		return m.renderExport(width)
	case screenOverview:
		return m.renderOverview(width)
	default:
		return m.renderMenu(width)
	}
}

// ---------------------------------------------------------------------------
// Header / footer.
// ---------------------------------------------------------------------------

func renderHeader(width int) string {
	title := appTitleStyle.Render("DESIGN")
	badge := badgeStyle.Render("Standalone TUI Design Kit")
	subtitle := hintStyle.Render("generic menu, add form, list table, edit form, settings and export screens")
	line := lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badge), subtitle)

	return panelStyle.Width(width).Render(line)
}

func renderFooter(width int, status string) string {
	content := lipgloss.JoinHorizontal(lipgloss.Center, statusLabelStyle.Render("STATUS"), " ", statusStyle.Render(status))

	return panelStyle.Width(width).Render(content)
}

// ---------------------------------------------------------------------------
// Menu: two-column layout of groups/buttons plus a dashboard-style summary.
// ---------------------------------------------------------------------------

func menuColumnLayout(groups []menuGroup) ([]int, []int) {
	leftColumn := make([]int, 0, len(groups)/2+1)
	rightColumn := make([]int, 0, len(groups)/2+1)

	for index := range groups {
		if index%2 == 0 {
			leftColumn = append(leftColumn, index)
		} else {
			rightColumn = append(rightColumn, index)
		}
	}

	return leftColumn, rightColumn
}

func menuColumnPosition(groupIndex int) (column int, row int) {
	if groupIndex%2 == 0 {
		return 0, groupIndex / 2
	}

	return 1, groupIndex / 2
}

func menuMoveWithinColumn(currentGroup int, direction int) (int, bool) {
	leftColumn, rightColumn := menuColumnLayout(appMenuGroups())
	column, row := menuColumnPosition(currentGroup)
	currentColumn := leftColumn
	if column == 1 {
		currentColumn = rightColumn
	}

	targetRow := row + direction
	if targetRow < 0 || targetRow >= len(currentColumn) {
		return 0, false
	}

	return currentColumn[targetRow], true
}

func menuMoveAcrossColumns(currentGroup int, direction int) (int, bool) {
	leftColumn, rightColumn := menuColumnLayout(appMenuGroups())
	column, row := menuColumnPosition(currentGroup)

	if direction < 0 {
		if column == 1 && row < len(leftColumn) {
			return leftColumn[row], true
		}

		return 0, false
	}

	if column == 0 && row < len(rightColumn) {
		return rightColumn[row], true
	}

	return 0, false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	groups := appMenuGroups()

	switch msg.String() {
	case "?":
		m.help.ShowAll = !m.help.ShowAll

		return m, nil
	case "left", "h", "shift+tab":
		if m.menuItem > 0 {
			m.menuItem--

			return m, nil
		}

		if targetGroup, ok := menuMoveAcrossColumns(m.menuGroup, -1); ok {
			m.menuGroup = targetGroup
			m.menuItem = 0
		}

		return m, nil
	case "right", "tab":
		if m.menuItem < len(groups[m.menuGroup].items)-1 {
			m.menuItem++

			return m, nil
		}

		if targetGroup, ok := menuMoveAcrossColumns(m.menuGroup, 1); ok {
			m.menuGroup = targetGroup
			m.menuItem = 0
		}

		return m, nil
	case "up":
		if targetGroup, ok := menuMoveWithinColumn(m.menuGroup, -1); ok {
			m.menuGroup = targetGroup
			m.menuItem = minInt(m.menuItem, len(groups[m.menuGroup].items)-1)
		}

		return m, nil
	case "down", "j":
		if targetGroup, ok := menuMoveWithinColumn(m.menuGroup, 1); ok {
			m.menuGroup = targetGroup
			m.menuItem = minInt(m.menuItem, len(groups[m.menuGroup].items)-1)
		}

		return m, nil
	case "enter":
		return m.activateMenuSelection()
	case "a":
		m.menuGroup = 1
		m.menuItem = 0

		return m.activateMenuSelection()
	case "e":
		m.menuGroup = 1
		m.menuItem = 1

		return m.activateMenuSelection()
	case "q":
		m.quitting = true

		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m model) activateMenuSelection() (tea.Model, tea.Cmd) {
	switch m.menuGroup {
	case 0:
		switch m.menuItem {
		case 0:
			m.screen = screenAddForm
			m.addForm = newAddItemForm(sampleCategoryOptions())
			m.status = "creating a new type A entry"
		case 1:
			m.screen = screenAddForm
			m.addForm = newAddItemForm(sampleCategoryOptions())
			m.status = "creating a new type B entry"
		case 2:
			m.screen = screenListTable
			m.listFilterMode = -2
			m.listTitle = "History"
			m.cursor = 0
			m.status = "history"
		case 3:
			m.screen = screenOverview
			m.status = "overview"
		}

		return m, nil
	case 1:
		switch m.menuItem {
		case 0:
			m.screen = screenAddForm
			m.addForm = newAddItemForm(sampleCategoryOptions())
			m.status = "add a new item"
		case 1:
			m.screen = screenListTable
			m.listFilterMode = -1
			m.listTitle = "All items"
			m.cursor = 0
			m.status = "all items"
		}

		return m, nil
	case 2, 3, 4, 5, 6:
		if m.menuItem == 0 {
			m.screen = screenAddForm
			m.addForm = newAddItemForm(sampleCategoryOptions())
			m.status = strings.ToLower(appMenuGroups()[m.menuGroup].title) + " / new entry"

			return m, nil
		}

		m.screen = screenListTable
		m.listFilterMode = m.menuItem - 1
		m.listTitle = appMenuGroups()[m.menuGroup].title + " / " + modeLabel(m.listFilterMode)
		m.cursor = 0
		m.status = strings.ToLower(m.listTitle)

		return m, nil
	case 7:
		switch m.menuItem {
		case 0:
			m.screen = screenSettings
			m.settingsCursor = 0
			m.settingsEditMode = settingsEditNone
			m.status = "settings"
		case 1:
			m.screen = screenExport
			m.status = "export"
		}

		return m, nil
	}

	return m, nil
}

func (m model) renderDashboard(width int) string {
	total := m.sumAmounts(m.items)
	ignoredCount := 0
	byMode := [3]int64{}

	for _, it := range m.items {
		if it.ignore {
			ignoredCount++
		} else {
			byMode[it.mode] += it.amountCents
		}
	}

	leftLines := []string{sectionTitleStyle.Render("Summary")}
	leftLines = append(leftLines,
		fmt.Sprintf("Total items:        %d", len(m.items)),
		fmt.Sprintf("Ignored items:      %d", ignoredCount),
		fmt.Sprintf("Total amount:       %s", renderAmount(total)),
		fmt.Sprintf("Net (A - B):        %s", renderSignedAmount(byMode[0]-byMode[1])),
	)
	leftColumn := strings.Join(leftLines, "\n")

	rightLines := []string{sectionTitleStyle.Render("Breakdown")}
	rightLines = append(rightLines,
		fmt.Sprintf("Mode A total:       %s", m.renderAmountConditionalRed(byMode[0])),
		fmt.Sprintf("Mode B total:       %s", m.renderAmountConditionalRed(byMode[1])),
		fmt.Sprintf("Mode C total:       %s", m.renderAmountConditionalRed(byMode[2])),
	)
	rightColumn := strings.Join(rightLines, "\n")

	columnWidth := (width - 8) / 2
	leftView := lipgloss.NewStyle().Width(columnWidth).Render(leftColumn)
	rightView := lipgloss.NewStyle().Width(columnWidth).Render(rightColumn)
	twoColumn := lipgloss.JoinHorizontal(lipgloss.Top, leftView, "  ", rightView)

	return panelStyle.Width(width).Render(twoColumn)
}

func (m model) renderMenu(width int) string {
	dashboard := m.renderDashboard(width)
	groups := appMenuGroups()
	leftColumn := make([]string, 0, len(groups)/2+1)
	rightColumn := make([]string, 0, len(groups)/2+1)

	for gi, group := range groups {
		block := m.renderMenuGroupBlock(gi, group)
		if gi%2 == 0 {
			leftColumn = append(leftColumn, block)
		} else {
			rightColumn = append(rightColumn, block)
		}
	}

	columnWidth := (width - 5) / 2
	leftView := lipgloss.NewStyle().Width(columnWidth).Render(strings.Join(leftColumn, "\n\n"))
	rightView := lipgloss.NewStyle().Width(columnWidth).Render(strings.Join(rightColumn, "\n\n"))
	twoColumnMenu := lipgloss.JoinHorizontal(lipgloss.Top, leftView, "  ", rightView)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dashboard,
		"",
		headlineStyle.Render("Menu"),
		"",
		twoColumnMenu,
		"",
		mutedStyle.Render("Use up/down to change groups, left/right (or Tab/Shift+Tab) to change buttons, Enter to open, ? for full help."),
		"",
		mutedStyle.Render("Press q to quit."),
		"",
		m.help.View(m.keys),
	)

	return panelStyle.Width(width).Render(content)
}

func (m model) renderMenuGroupBlock(groupIndex int, group menuGroup) string {
	buttons := make([]string, 0, len(group.items))

	for itemIndex, label := range group.items {
		style := buttonStyle
		if groupIndex == m.menuGroup && itemIndex == m.menuItem {
			style = buttonActiveStyle
		}

		buttons = append(buttons, style.Render(label))
	}

	return fieldLabelStyle.Render(group.title) + "\n" + strings.Join(buttons, " ")
}

// ---------------------------------------------------------------------------
// Add form.
// ---------------------------------------------------------------------------

func (m model) renderAddForm(width int) string {
	lines := []string{
		headlineStyle.Render("Add item"),
		mutedStyle.Render("Fill the fields, choose a category with left/right, and set whether this item should be ignored."),
		"",
	}

	nameLabel := fieldLabelStyle.Render("Name")
	nameValue := inputBoxStyle.Width(width - 18).Render(m.addForm.NameInput.View())
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, nameLabel, "  ", nameValue))

	descLabel := fieldLabelStyle.Render("Description")
	descValue := inputBoxStyle.Width(width - 18).Render(m.addForm.DescriptionInput.View())
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, descLabel, "  ", descValue))

	lines = append(lines, m.renderAddFormCategoryRow(width))

	amountLabel := fieldLabelStyle.Render("Amount")
	amountValue := inputBoxStyle.Width(width - 18).Render(m.addForm.AmountInput.View())
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, amountLabel, "  ", amountValue))

	ignorePrefix := "  "
	if m.addForm.Active == addFieldIgnore {
		ignorePrefix = "> "
	}
	ignoreMarker := "[ ]"
	if m.addForm.Ignore {
		ignoreMarker = "[x]"
	}
	lines = append(lines, ignorePrefix+fieldLabelStyle.Render("Ignore")+"  "+ignoreMarker)
	lines = append(lines, mutedStyle.Render("Ignored items stay out of the summary totals."))

	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Tab moves forward, Shift+Tab moves back, Space toggles the checkbox, Esc returns home."))

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderAddFormCategoryRow(width int) string {
	options := make([]string, 0, len(m.addForm.CategoryOptions))
	for index, option := range m.addForm.CategoryOptions {
		style := buttonStyle
		if index == m.addForm.CategoryIndex {
			style = buttonActiveStyle
		}

		options = append(options, style.Render(option))
	}

	prefix := " "
	if m.addForm.Active == addFieldCategory {
		prefix = ">"
	}

	label := fieldLabelStyle.Render(prefix + " Category")
	value := inputBoxStyle.Width(width - 18).Render(strings.Join(options, " "))

	return lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value)
}

func (m model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = fmt.Sprintf("loaded %d sample items", len(m.items))

		return m, nil
	case "up", "shift+tab":
		m.addForm = m.addForm.prev()

		return m, nil
	case "down", "tab":
		m.addForm = m.addForm.next()

		return m, nil
	case "left":
		if m.addForm.Active == addFieldCategory && m.addForm.CategoryIndex > 0 {
			m.addForm.CategoryIndex--
		}
		if m.addForm.Active == addFieldIgnore {
			m.addForm.Ignore = false
		}

		return m, nil
	case "right":
		if m.addForm.Active == addFieldCategory && m.addForm.CategoryIndex < len(m.addForm.CategoryOptions)-1 {
			m.addForm.CategoryIndex++
		}
		if m.addForm.Active == addFieldIgnore {
			m.addForm.Ignore = true
		}

		return m, nil
	case " ":
		if m.addForm.Active == addFieldIgnore {
			m.addForm.Ignore = !m.addForm.Ignore

			return m, nil
		}
	case "enter":
		if m.addForm.Active == addFieldIgnore {
			return m.saveItemFromForm()
		}

		m.addForm = m.addForm.next()

		return m, nil
	}

	if m.addForm.Active == addFieldCategory || m.addForm.Active == addFieldIgnore {
		return m, nil
	}

	var cmd tea.Cmd

	switch m.addForm.Active {
	case addFieldName:
		m.addForm.NameInput, cmd = m.addForm.NameInput.Update(msg)
	case addFieldDescription:
		m.addForm.DescriptionInput, cmd = m.addForm.DescriptionInput.Update(msg)
	case addFieldAmount:
		m.addForm.AmountInput, cmd = m.addForm.AmountInput.Update(msg)
	}

	return m, cmd
}

func (m model) saveItemFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addForm.NameInput.Value())
	description := strings.TrimSpace(m.addForm.DescriptionInput.Value())
	category := selectedStringOption(m.addForm.CategoryOptions, m.addForm.CategoryIndex)

	if name == "" {
		m.status = "name is required"

		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.addForm.AmountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	newItem := item{
		id:          m.nextID,
		name:        name,
		description: description,
		category:    category,
		mode:        m.nextID % 3,
		amountCents: amount,
		ignore:      m.addForm.Ignore,
		updatedAt:   time.Now(),
	}
	m.nextID++

	m.items = append([]item{newItem}, m.items...)
	m.addForm = newAddItemForm(sampleCategoryOptions())
	m.screen = screenMenu
	m.status = "saved item " + name

	return m, nil
}

// ---------------------------------------------------------------------------
// List table: sortable, filterable, with delete confirmation.
// ---------------------------------------------------------------------------

func sortFieldOptions() []string {
	return []string{"Amount (high to low)", "Name", "Last updated"}
}

func (m model) filteredItems() []item {
	result := make([]item, 0, len(m.items))

	for _, it := range m.items {
		switch {
		case m.listFilterMode == -1:
			result = append(result, it)
		case m.listFilterMode == -2:
			if it.ignore {
				result = append(result, it)
			}
		default:
			if !it.ignore && it.mode == m.listFilterMode {
				result = append(result, it)
			}
		}
	}

	sortItems(result, m.sortField)

	return result
}

func sortItems(items []item, field int) {
	sort.SliceStable(items, func(i, j int) bool {
		switch field {
		case 1:
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		case 2:
			return items[i].updatedAt.After(items[j].updatedAt)
		default:
			return items[i].amountCents > items[j].amountCents
		}
	})
}

func findItemIndex(items []item, id int) int {
	for i := range items {
		if items[i].id == id {
			return i
		}
	}

	return -1
}

func (m model) renderListTable(width int) string {
	title := m.listTitle
	if title == "" {
		title = "Items"
	}

	lines := []string{sectionTitleStyle.Render(title)}
	lines = append(lines, hintStyle.Render("Use up/down to move, Enter to edit, S changes sorting, Delete/Backspace asks confirmation, Esc to go back."))
	lines = append(lines, "")

	items := m.filteredItems()

	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render("No items in this view."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, fieldLabelStyle.Render("Sort")+"  "+sortFieldOptions()[m.sortField])
	if m.sortMenuOpen {
		lines = append(lines, m.renderSortMenu(width))
	}
	lines = append(lines, "")

	lines = append(lines, m.renderListHeader(width))

	for i, it := range items {
		lines = append(lines, m.renderListRow(width, i, it))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderListHeader(width int) string {
	nameWidth, categoryWidth, amountWidth, modeWidth, updatedWidth := 16, 12, 14, 10, 16
	descWidth := width - nameWidth - categoryWidth - amountWidth - modeWidth - updatedWidth - 12
	if descWidth < 20 {
		descWidth = 20
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", categoryWidth, "Category", amountWidth, "Amount", modeWidth, "Mode", updatedWidth, "Updated", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m model) renderListRow(width int, index int, it item) string {
	nameWidth, categoryWidth, amountWidth, modeWidth, updatedWidth := 16, 12, 14, 10, 16
	descWidth := width - nameWidth - categoryWidth - amountWidth - modeWidth - updatedWidth - 12
	if descWidth < 20 {
		descWidth = 20
	}

	prefix := " "
	style := rowStyle
	if index == m.cursor {
		prefix = ">"
		style = selectedRowStyle
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(it.name, nameWidth), categoryWidth, truncateText(it.category, categoryWidth), amountWidth, renderAmount(it.amountCents), modeWidth, modeLabel(it.mode), updatedWidth, formatUpdatedAt(it.updatedAt), descWidth, truncateText(it.description, descWidth))

	return style.Render(row)
}

func (m model) renderSortMenu(width int) string {
	lines := []string{
		fieldLabelStyle.Render("Sorting options"),
		mutedStyle.Render("Use up/down to choose. Enter applies. Esc closes."),
	}

	for index, option := range sortFieldOptions() {
		prefix := "  "
		if index == m.sortCursor {
			prefix = "> "
		}

		label := option
		if index == m.sortField {
			label += " (current)"
		}

		lines = append(lines, prefix+label)
	}

	menuWidth := width - 4
	if menuWidth < 32 {
		menuWidth = 32
	}

	return inputBoxStyle.Width(menuWidth).Render(strings.Join(lines, "\n"))
}

func (m model) updateListTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	items := m.filteredItems()

	if m.sortMenuOpen {
		switch strings.ToLower(msg.String()) {
		case "esc", "s":
			m.sortMenuOpen = false
			m.status = "sort unchanged"

			return m, nil
		case "up", "k":
			if m.sortCursor > 0 {
				m.sortCursor--
			}

			return m, nil
		case "down", "j":
			if m.sortCursor < len(sortFieldOptions())-1 {
				m.sortCursor++
			}

			return m, nil
		case "enter":
			selectedID := 0
			if m.cursor >= 0 && m.cursor < len(items) {
				selectedID = items[m.cursor].id
			}

			m.sortField = m.sortCursor
			m.sortMenuOpen = false

			reordered := m.filteredItems()
			if selectedID != 0 {
				m.cursor = findItemIndex(reordered, selectedID)
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.status = "sorting by " + sortFieldOptions()[m.sortField]

			return m, nil
		default:
			return m, nil
		}
	}

	if len(items) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = fmt.Sprintf("loaded %d sample items", len(m.items))
		}

		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = fmt.Sprintf("loaded %d sample items", len(m.items))

		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}

		return m, nil
	case "down":
		if m.cursor < len(items)-1 {
			m.cursor++
		}

		return m, nil
	case "s", "S":
		m.sortMenuOpen = true
		m.sortCursor = m.sortField
		m.status = "choose sorting"

		return m, nil
	case "enter":
		return m.beginEditForm(items[m.cursor]), nil
	case "backspace", "delete":
		selected := items[m.cursor]
		m = m.beginDeleteConfirmation(selected.id, selected.name)

		return m, nil
	default:
		return m, nil
	}
}

func (m model) beginDeleteConfirmation(id int, name string) model {
	m.deleteConfirmActive = true
	m.deleteConfirmID = id
	m.deleteConfirmName = strings.TrimSpace(name)

	displayName := m.deleteConfirmName
	if displayName == "" {
		displayName = fmt.Sprintf("id=%d", id)
	}

	m.status = fmt.Sprintf("confirm delete %s? press y to confirm, n to cancel", displayName)

	return m
}

func (m model) clearDeleteConfirmation(status string) model {
	m.deleteConfirmActive = false
	m.deleteConfirmID = 0
	m.deleteConfirmName = ""

	if strings.TrimSpace(status) != "" {
		m.status = status
	}

	return m
}

func (m model) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !m.deleteConfirmActive {
		return m, nil, false
	}

	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		updatedModel, cmd := m.applyConfirmedDelete()

		return updatedModel, cmd, true
	case "n", "esc":
		m = m.clearDeleteConfirmation("delete cancelled")

		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m model) applyConfirmedDelete() (tea.Model, tea.Cmd) {
	index := findItemIndex(m.items, m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("item not found")

		return m, nil
	}

	selected := m.items[index]
	m.items = append(m.items[:index], m.items[index+1:]...)
	delete(m.history, selected.id)

	filteredAfter := m.filteredItems()
	if len(filteredAfter) == 0 {
		m.cursor = 0
		m.screen = screenMenu
	} else if m.cursor >= len(filteredAfter) {
		m.cursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted " + selected.name)

	return m, nil
}

// ---------------------------------------------------------------------------
// Edit form: edit the amount/flags of a single item and log historical
// values against it, similar to the account "edit amount" screen.
// ---------------------------------------------------------------------------

func (m model) beginEditForm(selected item) model {
	m.editID = selected.id
	m.editActiveField = editFieldValue
	m.editUpdateLog = false
	m.editIgnore = selected.ignore

	m.editValueInput = textinput.New()
	m.editValueInput.Placeholder = "1234.56"
	m.editValueInput.CharLimit = 24
	m.editValueInput.Width = 20
	m.editValueInput.SetValue(formatAmount(selected.amountCents))
	m.editValueInput.Focus()

	m.editLogDateInput = textinput.New()
	m.editLogDateInput.Placeholder = "DD.MM.YYYY"
	m.editLogDateInput.CharLimit = 24
	m.editLogDateInput.Width = 20
	m.editLogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editLogDateInput.Blur()

	m.editLogValueInput = textinput.New()
	m.editLogValueInput.Placeholder = "1234.56"
	m.editLogValueInput.CharLimit = 24
	m.editLogValueInput.Width = 20
	m.editLogValueInput.SetValue(formatAmount(selected.amountCents))
	m.editLogValueInput.Blur()

	m.screen = screenEditForm
	m.status = "editing " + selected.name

	return m
}

func (m model) focusEditField() model {
	m.editValueInput.Blur()
	m.editLogDateInput.Blur()
	m.editLogValueInput.Blur()

	switch m.editActiveField {
	case editFieldValue:
		m.editValueInput.Focus()
	case editFieldLogDate:
		m.editLogDateInput.Focus()
	case editFieldLogValue:
		m.editLogValueInput.Focus()
	}

	return m
}

func (m model) renderEditForm(width int) string {
	index := findItemIndex(m.items, m.editID)
	if index < 0 {
		return panelStyle.Width(width).Render(mutedStyle.Render("No item selected."))
	}

	selected := m.items[index]

	prefixFor := func(field int) string {
		if m.editActiveField == field {
			return "> "
		}

		return "  "
	}

	updateLogMarker := "[ ]"
	if m.editUpdateLog {
		updateLogMarker = "[x]"
	}

	ignoreMarker := "[ ]"
	if m.editIgnore {
		ignoreMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit item"),
		mutedStyle.Render("Item: " + selected.name + " | " + selected.category + " | amount " + renderAmount(selected.amountCents)),
		"",
		prefixFor(editFieldValue) + fieldLabelStyle.Render("Amount"),
		inputBoxStyle.Width(24).Render(m.editValueInput.View()),
		prefixFor(editFieldUpdateLog) + fieldLabelStyle.Render("Update log on save") + "  " + updateLogMarker,
		prefixFor(editFieldIgnore) + fieldLabelStyle.Render("Ignore") + "  " + ignoreMarker,
		"",
		fieldLabelStyle.Render("Add historical value"),
		prefixFor(editFieldLogDate) + fieldLabelStyle.Render("Date") + "  " + m.editLogDateInput.View(),
		prefixFor(editFieldLogValue) + fieldLabelStyle.Render("Value") + "  " + m.editLogValueInput.View(),
		"",
		mutedStyle.Render("Enter on Amount saves the item. Enter on Value saves a log entry for Date."),
		mutedStyle.Render("Use up/down or tab/shift+tab to move fields. Space toggles checkboxes. Esc cancels."),
	}

	lines = append(lines, "", fieldLabelStyle.Render("Value history"))
	logEntries := m.history[selected.id]
	if len(logEntries) == 0 {
		lines = append(lines, mutedStyle.Render("No historical values yet."))
	} else {
		dateWidth, valueWidth := 12, 14
		header := fmt.Sprintf("%-*s %-*s", dateWidth, "Date", valueWidth, "Value")
		lines = append(lines, tableHeaderStyle.Render(header))

		for _, entry := range logEntries {
			row := fmt.Sprintf("%-*s %-*s", dateWidth, entry.day.Format("2006-01-02"), valueWidth, renderAmount(entry.valueCents))
			lines = append(lines, rowStyle.Render(row))
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) updateEditForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenListTable
		m.status = "edit cancelled"

		return m, nil
	case "up", "shift+tab":
		if m.editActiveField > 0 {
			m.editActiveField--
		}

		return m.focusEditField(), nil
	case "down", "tab":
		if m.editActiveField < editFieldCount-1 {
			m.editActiveField++
		}

		return m.focusEditField(), nil
	case " ":
		if m.editActiveField == editFieldUpdateLog {
			m.editUpdateLog = !m.editUpdateLog

			return m, nil
		}
		if m.editActiveField == editFieldIgnore {
			m.editIgnore = !m.editIgnore

			return m, nil
		}
	case "enter":
		switch m.editActiveField {
		case editFieldValue:
			return m.saveEditForm()
		case editFieldIgnore:
			m.editIgnore = !m.editIgnore

			return m, nil
		case editFieldLogValue:
			return m.applyLogValue()
		default:
			return m, nil
		}
	}

	var cmd tea.Cmd

	switch m.editActiveField {
	case editFieldValue:
		m.editValueInput, cmd = m.editValueInput.Update(msg)
	case editFieldLogDate:
		m.editLogDateInput, cmd = m.editLogDateInput.Update(msg)
	case editFieldLogValue:
		m.editLogValueInput, cmd = m.editLogValueInput.Update(msg)
	default:
		return m, nil
	}

	return m, cmd
}

func (m model) saveEditForm() (tea.Model, tea.Cmd) {
	index := findItemIndex(m.items, m.editID)
	if index < 0 {
		m.status = "no item selected"

		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editValueInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	now := time.Now()
	logWarn := ""

	if m.editUpdateLog {
		m.history[m.editID] = append(m.history[m.editID], historyEntry{day: now, valueCents: amount})
	}

	selected := m.items[index]
	selected.amountCents = amount
	selected.ignore = m.editIgnore
	selected.updatedAt = now
	m.items[index] = selected

	m.screen = screenListTable
	m.status = "updated " + selected.name + logWarn

	return m, nil
}

func (m model) applyLogValue() (tea.Model, tea.Cmd) {
	index := findItemIndex(m.items, m.editID)
	if index < 0 {
		m.status = "no item selected"

		return m, nil
	}

	rawDay := strings.TrimSpace(m.editLogDateInput.Value())
	if rawDay == "" {
		m.status = "log date is required"

		return m, nil
	}

	day, err := time.ParseInLocation("02.01.2006", rawDay, time.Now().Location())
	if err != nil {
		m.status = "log date must use DD.MM.YYYY format"

		return m, nil
	}

	value, err := parseAmountCents(strings.TrimSpace(m.editLogValueInput.Value()))
	if err != nil {
		m.status = "log value error: " + err.Error()

		return m, nil
	}

	m.history[m.editID] = append(m.history[m.editID], historyEntry{day: day, valueCents: value})
	m.status = "saved log value for " + day.Format("2006-01-02")

	return m, nil
}

// ---------------------------------------------------------------------------
// Settings: an editable list of rows grouped into sections, mirroring the
// "select row, Enter edits it inline" pattern from the source app.
// ---------------------------------------------------------------------------

func (m model) settingsCategoryAddCursor() int {
	return len(m.settingsCategories) + 1
}

func (m model) renderSettings(width int) string {
	lines := []string{sectionTitleStyle.Render("Settings"), hintStyle.Render("Use up/down to select rows. Enter edits selected row. Esc returns to menu."), ""}
	lines = append(lines, sectionTitleStyle.Render("General"))

	optionPrefix := " "
	if m.settingsCursor == 0 {
		optionPrefix = ">"
	}

	optionValue := m.settingsOption
	if m.settingsEditMode == settingsEditOption {
		optionValue = m.settingsEditInput.View()
	}

	lines = append(lines, fmt.Sprintf("%s %-18s %s", optionPrefix, "option_one", optionValue), "", sectionTitleStyle.Render("Categories"))

	for index, category := range m.settingsCategories {
		prefix := " "
		if m.settingsCursor == index+1 {
			prefix = ">"
		}

		value := category
		if m.settingsEditMode == settingsEditCategory && m.settingsEditIndex == index {
			value = m.settingsEditInput.View()
		}

		lines = append(lines, fmt.Sprintf("%s %s", prefix, value))
	}

	addPrefix := " "
	if m.settingsCursor == m.settingsCategoryAddCursor() {
		addPrefix = ">"
	}

	addValue := "+ add category"
	if m.settingsEditMode == settingsEditCategory && m.settingsEditIndex == -1 {
		addValue = m.settingsEditInput.View()
	}

	lines = append(lines, fmt.Sprintf("%s %s", addPrefix, addValue))

	if m.settingsEditMode != settingsEditNone {
		lines = append(lines, "", mutedStyle.Render("Enter saves the row. Esc cancels the edit."))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) settingsMaxCursor() int {
	return m.settingsCategoryAddCursor()
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsEditMode != settingsEditNone {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.status = "edit cancelled"

			return m, nil
		case "enter":
			return m.saveSettingsEdit()
		}

		var cmd tea.Cmd
		m.settingsEditInput, cmd = m.settingsEditInput.Update(msg)

		return m, cmd
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = "settings closed"

		return m, nil
	case "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}

		return m, nil
	case "down":
		if m.settingsCursor < m.settingsMaxCursor() {
			m.settingsCursor++
		}

		return m, nil
	case "enter":
		return m.beginSettingsEdit()
	default:
		return m, nil
	}
}

func (m model) beginSettingsEdit() (tea.Model, tea.Cmd) {
	m.settingsEditInput = textinput.New()
	m.settingsEditInput.CharLimit = 40
	m.settingsEditInput.Width = 28
	m.settingsEditInput.Focus()

	switch {
	case m.settingsCursor == 0:
		m.settingsEditMode = settingsEditOption
		m.settingsEditInput.SetValue(m.settingsOption)
	case m.settingsCursor == m.settingsCategoryAddCursor():
		m.settingsEditMode = settingsEditCategory
		m.settingsEditIndex = -1
		m.settingsEditInput.Placeholder = "New category"
	default:
		m.settingsEditMode = settingsEditCategory
		m.settingsEditIndex = m.settingsCursor - 1
		m.settingsEditInput.SetValue(m.settingsCategories[m.settingsEditIndex])
	}

	return m, textinput.Blink
}

func (m model) saveSettingsEdit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.settingsEditInput.Value())

	if value == "" {
		m.status = "value cannot be empty"

		return m, nil
	}

	switch m.settingsEditMode {
	case settingsEditOption:
		m.settingsOption = value
		m.status = "saved option_one"
	case settingsEditCategory:
		if m.settingsEditIndex == -1 {
			m.settingsCategories = append(m.settingsCategories, value)
			m.status = "added category " + value
		} else {
			m.settingsCategories[m.settingsEditIndex] = value
			m.status = "updated category " + value
		}
	}

	m.settingsEditMode = settingsEditNone

	return m, nil
}

// ---------------------------------------------------------------------------
// Export: dataset/format buttons, a path field and a run button.
// ---------------------------------------------------------------------------

const (
	exportFieldDataset = iota
	exportFieldFormat
	exportFieldPath
	exportFieldRun
	exportFieldCount
)

func exportDatasetOptions() []string {
	return []string{"Dataset A", "Dataset B"}
}

func exportFormatOptions() []string {
	return []string{"Format A", "Format B", "Format C"}
}

func (m model) renderExport(width int) string {
	datasetPrefix, formatPrefix, pathPrefix := " ", " ", " "
	runStyle := buttonStyle

	switch m.exportActive {
	case exportFieldDataset:
		datasetPrefix = ">"
	case exportFieldFormat:
		formatPrefix = ">"
	case exportFieldPath:
		pathPrefix = ">"
	case exportFieldRun:
		runStyle = buttonActiveStyle
	}

	formatOptions := make([]string, 0, len(exportFormatOptions()))
	for i, option := range exportFormatOptions() {
		style := buttonStyle
		if i == m.exportFormatIndex {
			style = buttonActiveStyle
		}
		formatOptions = append(formatOptions, style.Render(option))
	}

	rows := []string{
		sectionTitleStyle.Render("Export"),
		hintStyle.Render("Use up/down to change fields. Left/right changes options. Enter exports from the Export button. Esc returns to menu."),
		"",
		fmt.Sprintf("%s %-8s %s", datasetPrefix, "Data", buttonActiveStyle.Render(exportDatasetOptions()[m.exportDatasetIndex])),
		fmt.Sprintf("%s %-8s %s", formatPrefix, "Format", strings.Join(formatOptions, " ")),
		fmt.Sprintf("%s %-8s %s", pathPrefix, "Path", m.exportPathInput.View()),
		"",
		runStyle.Render("Export"),
		"",
		mutedStyle.Render("This demo screen does not write any files; it only reports what would happen."),
	}

	return panelStyle.Width(width).Render(strings.Join(rows, "\n"))
}

func (m model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = "export closed"

		return m, nil
	case "up", "shift+tab":
		if m.exportActive > 0 {
			m.exportActive--
		}

		return m, nil
	case "down", "tab":
		if m.exportActive < exportFieldCount-1 {
			m.exportActive++
		}

		return m, nil
	case "left":
		if m.exportActive == exportFieldDataset && m.exportDatasetIndex > 0 {
			m.exportDatasetIndex--
		}
		if m.exportActive == exportFieldFormat && m.exportFormatIndex > 0 {
			m.exportFormatIndex--
		}

		return m, nil
	case "right":
		if m.exportActive == exportFieldDataset && m.exportDatasetIndex < len(exportDatasetOptions())-1 {
			m.exportDatasetIndex++
		}
		if m.exportActive == exportFieldFormat && m.exportFormatIndex < len(exportFormatOptions())-1 {
			m.exportFormatIndex++
		}

		return m, nil
	case "enter":
		if m.exportActive == exportFieldRun {
			m.status = fmt.Sprintf("exported %s as %s to %s (demo, no file written)", exportDatasetOptions()[m.exportDatasetIndex], exportFormatOptions()[m.exportFormatIndex], m.exportPathInput.Value())

			return m, nil
		}

		m.exportActive++
		if m.exportActive >= exportFieldCount {
			m.exportActive = exportFieldCount - 1
		}

		return m, nil
	}

	if m.exportActive != exportFieldPath {
		return m, nil
	}

	var cmd tea.Cmd
	m.exportPathInput, cmd = m.exportPathInput.Update(msg)

	return m, cmd
}

// ---------------------------------------------------------------------------
// Overview: a report grouping items by category with progress bars.
// ---------------------------------------------------------------------------

func (m model) updateOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenMenu
		m.status = fmt.Sprintf("loaded %d sample items", len(m.items))
	}

	return m, nil
}

func (m model) renderOverview(width int) string {
	lines := []string{sectionTitleStyle.Render("Overview"), hintStyle.Render("Esc returns to menu."), ""}

	totals := map[string]int64{}
	order := make([]string, 0)

	for _, it := range m.items {
		if _, seen := totals[it.category]; !seen {
			order = append(order, it.category)
		}
		totals[it.category] += it.amountCents
	}

	grandTotal := m.sumAmounts(m.items)

	for _, category := range order {
		amount := totals[category]
		label := fmt.Sprintf("%-14s %-14s", truncateText(category, 14), renderAmount(amount))
		bar := renderProgressBar(amount, grandTotal, 30)
		lines = append(lines, rowStyle.Render(label)+"  "+bar)
	}

	lines = append(lines, "", fieldLabelStyle.Render("Grand total: ")+renderAmount(grandTotal))

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func renderProgressBar(paid int64, total int64, width int) string {
	if width < 8 {
		width = 8
	}

	if total <= 0 {
		bar := progressRestStyle.Render(strings.Repeat("░", width))
		return bar + " 0.0%"
	}

	if paid < 0 {
		paid = 0
	}
	if paid > total {
		paid = total
	}

	ratio := float64(paid) / float64(total)
	filled := int(math.Round(ratio * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := progressFillStyle.Render(strings.Repeat("█", filled)) + progressRestStyle.Render(strings.Repeat("░", width-filled))

	return fmt.Sprintf("%s %5.1f%%", bar, ratio*100)
}

// ---------------------------------------------------------------------------
// Small formatting helpers.
// ---------------------------------------------------------------------------

func (m model) sumAmounts(items []item) int64 {
	var total int64

	for _, it := range items {
		if it.ignore {
			continue
		}

		total += it.amountCents
	}

	return total
}

func (m model) renderAmountConditionalRed(cents int64) string {
	formatted := renderAmount(cents)
	if cents > 200000 {
		return negativeStyle.Render(formatted)
	}

	return formatted
}

func renderAmount(cents int64) string {
	sign := ""
	absoluteCents := cents

	if cents < 0 {
		sign = "-"
		absoluteCents = -cents
	}

	return sign + "$" + formatAmount(absoluteCents)
}

func renderSignedAmount(cents int64) string {
	formatted := renderAmount(cents)
	if cents > 0 {
		return positiveStyle.Render(formatted)
	}
	if cents < 0 {
		return negativeStyle.Render(formatted)
	}

	return formatted
}

func selectedStringOption(options []string, index int) string {
	if len(options) == 0 {
		return ""
	}

	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func formatAmount(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

func formatUpdatedAt(value time.Time) string {
	if value.IsZero() || value.Year() < 1971 {
		return "1970-01-01"
	}

	return value.Local().Format("2006-01-02 15:04")
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}

	if len(value) <= limit {
		return value
	}

	if limit <= 1 {
		return value[:limit]
	}

	return value[:limit-1] + "…"
}

func parseAmountCents(raw string) (int64, error) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("amount must be a number")
	}

	if amount < 0 {
		return 0, fmt.Errorf("amount cannot be negative")
	}

	return int64(math.Round(amount * 100)), nil
}
