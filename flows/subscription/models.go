package subscription

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type Subscription struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name              string
	Currency          string
	AmountCents       int64
	Period            string
	PaymentMethod     string
	Type              string
	IsActive          bool
	PaymentDateYearly string
	PaymentDayMonthly *int
}

const (
	SubFieldName = iota
	SubFieldCurrency
	SubFieldAmount
	SubFieldPeriod
	SubFieldPaymentMethodChoice
	SubFieldPaymentMethod
	SubFieldType
	SubFieldIsActive
	SubFieldDayYearly
	SubFieldDayMonthly
	SubFieldCount
)

const (
	EditSubFieldAmount = iota
	EditSubFieldPaymentMethod
	EditSubFieldIsActive
	EditSubFieldCount
)

// AddSubscriptionForm is the interface model for TUI.
type AddSubscriptionForm struct {
	Inputs               []textinput.Model
	Active               int
	CurrencyOptions      []string
	CurrencyIndex        int
	PaymentMethodOptions []string
	PaymentMethodIndex   int
	PeriodOptions        []string
	PeriodIndex          int
	TypeOptions          []string
	TypeIndex            int
	IsActive             bool
}

type EditSubscriptionForm struct {
	AmountInput        textinput.Model
	PaymentMethodInput textinput.Model
	IsActive           bool
	ActiveField        int
	PeriodLabel        string
	TypeLabel          string
	CurrencyLabel      string
	NameLabel          string
	PaymentDateYearly  string
	PaymentDayMonthly  string
}
