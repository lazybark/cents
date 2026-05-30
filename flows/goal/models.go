package goal

import "time"

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
