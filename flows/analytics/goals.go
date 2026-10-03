package analytics

import (
	"math"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/goal"
)

// Goal pace statuses.
const (
	PaceOnTrack  = "on-track"
	PaceBehind   = "behind"
	PaceNoTarget = "no-target"
	PaceStalled  = "stalled"
)

// paceMonths is how far back the recent pace looks.
const paceMonths = 6

const daysPerMonth = 365.25 / 12

// GoalPace is how an unfinished goal is going at its recent pace: what's
// added a month on average lately, when the target would be reached at
// that pace (Projected), and how that compares with the target date
// (MonthsLate, negative when early). NeededPerMonth is what reaching it by
// the target date takes, when that date is still ahead.
type GoalPace struct {
	ID             uint
	Name           string
	Currency       string
	LeftCents      int64
	TargetDate     *time.Time
	PerMonthCents  int64
	Projected      *time.Time
	MonthsLate     int
	NeededPerMonth int64
	Status         string
}

// Pace works out each unfinished goal's pace from what was added over the
// last months (since the goal started, if later; at least a month), going
// by its log; a goal without a log goes by what it has since it started.
func Pace(goals []goal.Goal, logs []goal.GoalLog, now time.Time) []GoalPace {
	byGoal := map[uint][]goal.GoalLog{}
	for _, l := range logs {
		byGoal[l.GoalID] = append(byGoal[l.GoalID], l)
	}

	today := dates.Day(now)
	result := []GoalPace{}
	for _, g := range goals {
		if g.IsDone() {
			continue
		}

		from := today.AddDate(0, -paceMonths, 0)
		if started := dates.Day(g.DateStartedAt); started.After(from) {
			from = started
		}

		var added int64
		if goalLogs := byGoal[g.ID]; len(goalLogs) > 0 {
			for _, l := range goalLogs {
				if !dates.Day(l.CreatedAt).Before(from) {
					added += l.DeltaAccumulatedCents
				}
			}
		} else {
			from = dates.Day(g.DateStartedAt)
			added = g.AmountAccumulatedCents
		}

		days := math.Max(today.Sub(from).Hours()/24, daysPerMonth)
		perDay := float64(added) / days
		p := GoalPace{ID: g.ID, Name: g.Name, Currency: g.Currency, LeftCents: g.LeftCents(), TargetDate: g.TargetDate, PerMonthCents: int64(math.Round(perDay * daysPerMonth))}

		if g.TargetDate != nil {
			if monthsLeft := dates.Day(*g.TargetDate).Sub(today).Hours() / 24 / daysPerMonth; monthsLeft > 0 {
				p.NeededPerMonth = int64(math.Ceil(float64(p.LeftCents) / math.Max(monthsLeft, 1)))
			}
		}

		switch {
		case perDay <= 0:
			p.Status = PaceStalled
		default:
			projected := today.AddDate(0, 0, int(math.Ceil(float64(p.LeftCents)/perDay)))
			p.Projected = &projected
			p.Status = PaceNoTarget
			if g.TargetDate != nil {
				late := projected.Sub(dates.Day(*g.TargetDate)).Hours() / 24 / daysPerMonth
				p.MonthsLate = int(math.Round(late))
				p.Status = PaceOnTrack
				if late > 0 {
					p.MonthsLate = int(math.Ceil(late))
					p.Status = PaceBehind
				}
			}
		}

		result = append(result, p)
	}

	return result
}
