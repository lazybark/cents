package goal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/money"
)

// ListMode picks which goals a list shows, like the TUI's goal menu.
type ListMode string

const (
	ListActive ListMode = "active"
	ListDone   ListMode = "done"
)

const defaultDepositNote = "manual accumulated adjustment"

// New validates a new goal and builds it. Every interface creates goals
// through here, so the rules and messages match; dates are typed in format.
func New(name, currency, target, accumulated, description, dateStarted, targetDate string, format dates.Format, now time.Time) (Goal, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Goal{}, errors.New("goal name is required")
	}

	g := Goal{Name: name, Currency: strings.TrimSpace(currency)}

	return g.Edit(target, accumulated, dateStarted, targetDate, description, format, now)
}

// Edit applies the fields that can change after creation. Name and currency
// stay as they are.
func (g Goal) Edit(target, accumulated, dateStarted, targetDate, description string, format dates.Format, now time.Time) (Goal, error) {
	targetCents, err := money.ParseAmountCents(target)
	if err != nil {
		return Goal{}, fmt.Errorf("target amount error: %w", err)
	}

	if targetCents <= 0 {
		return Goal{}, errors.New("target amount must be greater than zero")
	}

	accumulatedCents, err := money.ParseAmountCents(accumulated)
	if err != nil {
		return Goal{}, fmt.Errorf("accumulated amount error: %w", err)
	}

	if accumulatedCents > targetCents {
		return Goal{}, errors.New("accumulated amount cannot be more than target")
	}

	started, err := format.Required(dateStarted, "start")
	if err != nil {
		return Goal{}, err
	}

	until, err := format.Optional(targetDate, "target")
	if err != nil {
		return Goal{}, err
	}

	g.TargetAmountCents = targetCents
	g.AmountAccumulatedCents = accumulatedCents
	g.DateStartedAt = started
	g.TargetDate = until
	g.Description = strings.TrimSpace(description)
	g.LastUpdatedAt = now

	return g, nil
}

// ApplyDelta adds delta (signed, like "-10") to the accumulated amount and
// returns the updated goal with the log entry to store alongside it. The
// accumulated amount has to stay between zero and the target.
func (g Goal) ApplyDelta(delta, date, note string, format dates.Format, now time.Time) (Goal, GoalLog, error) {
	deltaCents, err := money.ParseSignedAmountCents(delta)
	if err != nil {
		return Goal{}, GoalLog{}, fmt.Errorf("log delta error: %w", err)
	}

	if deltaCents == 0 {
		return Goal{}, GoalLog{}, errors.New("delta cannot be zero")
	}

	next := g.AmountAccumulatedCents + deltaCents
	if next < 0 || next > g.TargetAmountCents {
		return Goal{}, GoalLog{}, errors.New("delta makes accumulated amount out of range")
	}

	when, err := format.LogTime(date, now)
	if err != nil {
		return Goal{}, GoalLog{}, err
	}

	note = strings.TrimSpace(note)
	if note == "" {
		note = defaultDepositNote
	}

	g.AmountAccumulatedCents = next
	g.LastUpdatedAt = now

	return g, GoalLog{GoalID: g.ID, DeltaAccumulatedCents: deltaCents, Note: note, CreatedAt: when}, nil
}

// RemoveDelta undoes a logged change: the accumulated amount moves back by
// its delta. The result has to stay between zero and the target, which it
// may not if the accumulated amount was edited by hand since.
func (g Goal) RemoveDelta(entry GoalLog, now time.Time) (Goal, error) {
	if entry.GoalID != g.ID {
		return Goal{}, errors.New("log entry belongs to another goal")
	}

	next := g.AmountAccumulatedCents - entry.DeltaAccumulatedCents
	if next < 0 || next > g.TargetAmountCents {
		return Goal{}, errors.New("removing this entry makes accumulated amount out of range; edit accumulated instead")
	}

	g.AmountAccumulatedCents = next
	g.LastUpdatedAt = now

	return g, nil
}

// IsDone reports whether the target is reached; done goals move to history.
func (g Goal) IsDone() bool {
	return g.AmountAccumulatedCents >= g.TargetAmountCents
}

// LeftCents is what is still to be saved, never below zero.
func (g Goal) LeftCents() int64 {
	return max(g.TargetAmountCents-g.AmountAccumulatedCents, 0)
}

// Percent is how much of the target is saved, between 0 and 100.
func (g Goal) Percent() float64 {
	if g.TargetAmountCents <= 0 {
		return 0
	}

	return min(max(float64(g.AmountAccumulatedCents)/float64(g.TargetAmountCents)*100, 0), 100)
}

// Filter keeps the goals a list mode shows: active ones or done ones.
func Filter(items []Goal, mode ListMode) []Goal {
	filtered := make([]Goal, 0, len(items))
	for _, item := range items {
		if item.IsDone() == (mode == ListDone) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}
