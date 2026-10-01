package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/goal"
)

var errGoalNotFound = errors.New("goal not found")

type GoalRow struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	Currency         string  `json:"currency"`
	TargetCents      int64   `json:"targetCents"`
	AccumulatedCents int64   `json:"accumulatedCents"`
	LeftCents        int64   `json:"leftCents"`
	Percent          float64 `json:"percent"`
	StartedAt        string  `json:"startedAt"`
	TargetDate       string  `json:"targetDate"`
	Overdue          bool    `json:"overdue"`
	Description      string  `json:"description"`
}

// GoalsView is one goal list. Progress counts accumulated amounts as paid,
// in the base currency.
type GoalsView struct {
	BaseCurrency string         `json:"baseCurrency"`
	Mode         string         `json:"mode"`
	Goals        []GoalRow      `json:"goals"`
	Progress     Progress       `json:"progress"`
	Counts       map[string]int `json:"counts"`
	Currencies   []string       `json:"currencies"`
}

type NewGoalInput struct {
	Name        string `json:"name"`
	Currency    string `json:"currency"`
	Target      string `json:"target"`
	Accumulated string `json:"accumulated"`
	StartedAt   string `json:"startedAt"`
	TargetDate  string `json:"targetDate"`
	Description string `json:"description"`
}

type GoalUpdateInput struct {
	ID          uint   `json:"id"`
	Target      string `json:"target"`
	Accumulated string `json:"accumulated"`
	StartedAt   string `json:"startedAt"`
	TargetDate  string `json:"targetDate"`
	Description string `json:"description"`
}

// Goals lists "active" goals or "done" ones (target reached), like the
// TUI's active goals and goal history.
func (a *API) Goals(mode string) (GoalsView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return GoalsView{}, err
	}

	items, err := storage.LoadGoals()
	if err != nil {
		return GoalsView{}, fmt.Errorf("failed to load goals: %w", err)
	}

	listMode := goal.ListMode(mode)
	if listMode != goal.ListDone {
		listMode = goal.ListActive
	}

	listed := goal.Filter(items, listMode)
	accumulated, target := goal.ProgressInBaseCents(listed, stts)
	now := time.Now()

	view := GoalsView{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Mode:         string(listMode),
		Goals:        make([]GoalRow, 0, len(listed)),
		Progress:     Progress{PaidCents: accumulated, TotalCents: target},
		Counts: map[string]int{
			string(goal.ListActive): len(goal.Filter(items, goal.ListActive)),
			string(goal.ListDone):   len(goal.Filter(items, goal.ListDone)),
		},
		Currencies: stts.CurrencyOptions(),
	}

	for _, item := range listed {
		view.Goals = append(view.Goals, goalRow(item, now))
	}

	return view, nil
}

func (a *API) CreateGoal(input NewGoalInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	entry, err := goal.New(input.Name, input.Currency, input.Target, input.Accumulated, input.Description, input.StartedAt, input.TargetDate, dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(input.Currency))
	if !ok {
		return CreatedIn{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
	}

	entry.Currency = currency

	if err := storage.CreateGoal(&entry); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	mode := goal.ListActive
	if entry.IsDone() {
		mode = goal.ListDone
	}

	return CreatedIn{Mode: string(mode)}, nil
}

// UpdateGoal changes target, accumulated amount, dates and description;
// name and currency can't change, as in the TUI.
func (a *API) UpdateGoal(input GoalUpdateInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	current, err := findGoal(storage, input.ID)
	if err != nil {
		return err
	}

	updated, err := current.Edit(input.Target, input.Accumulated, input.StartedAt, input.TargetDate, input.Description, dates.ISO, time.Now())
	if err != nil {
		return err
	}

	if err := storage.SaveGoal(&updated); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

// GoalLogs returns a goal's latest logged changes, newest first.
func (a *API) GoalLogs(id uint) ([]PaymentLog, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadGoalLogs(id)
	if err != nil {
		return nil, err
	}

	result := make([]PaymentLog, 0, len(logs))
	for _, entry := range logs {
		result = append(result, PaymentLog{ID: entry.ID, When: entry.CreatedAt.Local().Format("2006-01-02 15:04"), DeltaCents: entry.DeltaAccumulatedCents, Note: entry.Note})
	}

	return result, nil
}

// AddGoalChange logs a change and moves the accumulated amount by it,
// returning the goal as it is now.
func (a *API) AddGoalChange(input PaymentInput) (GoalRow, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return GoalRow{}, err
	}

	current, err := findGoal(storage, input.ID)
	if err != nil {
		return GoalRow{}, err
	}

	now := time.Now()
	updated, entry, err := current.ApplyDelta(input.Delta, input.Date, input.Note, dates.ISO, now)
	if err != nil {
		return GoalRow{}, err
	}

	if err := storage.SaveGoal(&updated); err != nil {
		return GoalRow{}, fmt.Errorf("goal update failed: %w", err)
	}

	if err := storage.CreateGoalLog(&entry); err != nil {
		return GoalRow{}, fmt.Errorf("log save failed: %w", err)
	}

	return goalRow(updated, now), nil
}

// DeleteGoalChange removes one logged change and undoes it, so the
// accumulated amount moves back by it. Only the changes GoalLogs lists can
// be removed. Returns the goal as it is now.
func (a *API) DeleteGoalChange(goalID uint, logID uint) (GoalRow, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return GoalRow{}, err
	}

	current, err := findGoal(storage, goalID)
	if err != nil {
		return GoalRow{}, err
	}

	logs, err := storage.LoadGoalLogs(goalID)
	if err != nil {
		return GoalRow{}, err
	}

	var change *goal.GoalLog
	for i := range logs {
		if logs[i].ID == logID {
			change = &logs[i]
		}
	}

	if change == nil {
		return GoalRow{}, errLogNotFound
	}

	now := time.Now()
	updated, err := current.RemoveDelta(*change, now)
	if err != nil {
		return GoalRow{}, err
	}

	if err := storage.DeleteGoalLog(&updated, logID); err != nil {
		return GoalRow{}, err
	}

	return goalRow(updated, now), nil
}

func (a *API) DeleteGoal(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findGoal(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteGoal(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func findGoal(storage StorageWorker, id uint) (goal.Goal, error) {
	items, err := storage.LoadGoals()
	if err != nil {
		return goal.Goal{}, fmt.Errorf("failed to load goals: %w", err)
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return goal.Goal{}, errGoalNotFound
}

func goalRow(item goal.Goal, now time.Time) GoalRow {
	return GoalRow{
		ID:               item.ID,
		Name:             item.Name,
		Currency:         item.Currency,
		TargetCents:      item.TargetAmountCents,
		AccumulatedCents: item.AmountAccumulatedCents,
		LeftCents:        item.LeftCents(),
		Percent:          item.Percent(),
		StartedAt:        formatDay(item.DateStartedAt),
		TargetDate:       formatOptionalDay(item.TargetDate),
		Overdue:          overdue(item.TargetDate, item.IsDone(), now),
		Description:      item.Description,
	}
}
