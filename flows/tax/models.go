package tax

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lazybark/cents/flows/settings"
)

type Tax struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	TaxTypeID       uint
	TaxCountry      string
	TaxTypeName     string
	AmountDueCents  int64
	AmountPaidCents int64
	Period          string
	DueDate         *time.Time
	Comment         string
}

type TaxLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	TaxID          uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}

const (
	TaxFieldTaxType = iota
	TaxFieldAmountDue
	TaxFieldAmountPaid
	TaxFieldPeriod
	TaxFieldDueDate
	TaxFieldComment
	TaxFieldCount
)

const (
	EditTaxFieldAmountDue = iota
	EditTaxFieldAmountPaid
	EditTaxFieldPeriod
	EditTaxFieldDueDate
	EditTaxFieldComment
	EditTaxFieldLogDelta
	EditTaxFieldLogDate
	EditTaxFieldLogComment
	EditTaxFieldCount
)

type AddTaxForm struct {
	Inputs          []textinput.Model
	Active          int
	TaxTypeOptions  []settings.SettingTaxType
	TaxTypeIndex    int
	TaxDisplayNames []string
}

type EditTaxForm struct {
	AmountDueInput  textinput.Model
	AmountPaidInput textinput.Model
	PeriodInput     textinput.Model
	DueDateInput    textinput.Model
	CommentInput    textinput.Model
	LogDeltaInput   textinput.Model
	LogDateInput    textinput.Model
	LogCommentInput textinput.Model
	ActiveField     int
	TaxTypeLabel    string
	CountryLabel    string
}
