package goal

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type Goal struct {
	ID                     uint `gorm:"primaryKey"`
	CreatedAt              time.Time
	LastUpdatedAt          time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name                   string
	Currency               string
	TargetAmountCents      int64
	AmountAccumulatedCents int64
	Description            string
	DateStartedAt          time.Time
	TargetDate             *time.Time
}

type GoalLog struct {
	ID                    uint `gorm:"primaryKey"`
	CreatedAt             time.Time
	GoalID                uint `gorm:"index;not null"`
	DeltaAccumulatedCents int64
	Note                  string
}

const (
	GoalFieldName = iota
	GoalFieldCurrency
	GoalFieldTargetAmount
	GoalFieldAccumulated
	GoalFieldDescription
	GoalFieldDateStarted
	GoalFieldTargetDate
	GoalFieldCount
)

const (
	EditGoalFieldTargetAmount = iota
	EditGoalFieldAccumulated
	EditGoalFieldDateStarted
	EditGoalFieldTargetDate
	EditGoalFieldDescription
	EditGoalFieldLogDelta
	EditGoalFieldLogDate
	EditGoalFieldLogComment
	EditGoalFieldCount
)

type AddGoalForm struct {
	Inputs          []textinput.Model
	Active          int
	CurrencyOptions []string
	CurrencyIndex   int
}
