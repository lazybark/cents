package debt

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type Debt struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Peer            string
	Currency        string
	AmountCents     int64
	AmountPaidCents int64
	IsOwedToUser    bool
	DebtCreatedAt   time.Time
	DueDate         *time.Time
	Comment         string
}

type DebtLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	DebtID         uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}

const (
	EditDebtFieldAmount = iota
	EditDebtFieldAmountPaid
	EditDebtFieldDebtCreated
	EditDebtFieldDueDate
	EditDebtFieldComment
	EditDebtFieldLogDelta
	EditDebtFieldLogDate
	EditDebtFieldLogComment
	EditDebtFieldCount
)

const (
	DebtFieldDirection = iota
	DebtFieldPeer
	DebtFieldCurrency
	DebtFieldAmount
	DebtFieldAmountPaid
	DebtFieldDebtCreated
	DebtFieldDueDate
	DebtFieldComment
	DebtFieldCount
)

type AddDebtForm struct {
	Inputs          []textinput.Model
	Active          int
	CurrencyOptions []string
	CurrencyIndex   int
	IsOwedToUser    bool
}

type EditDebtForm struct {
	AmountInput      textinput.Model
	AmountPaidInput  textinput.Model
	DebtCreatedInput textinput.Model
	DueDateInput     textinput.Model
	CommentInput     textinput.Model
	LogDeltaInput    textinput.Model
	LogDateInput     textinput.Model
	LogCommentInput  textinput.Model
	ActiveField      int
	PeerLabel        string
	CurrencyLabel    string
	DirectionLabel   string
}
