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
	// NextPaymentDate is a payment date the schedule repeats from, every
	// period. Without one, PaymentDateYearly or PaymentDayMonthly (what the
	// TUI sets) stand in for it; see Anchor.
	NextPaymentDate *time.Time
	// LastPaidDate is the latest payment marked paid (the latest
	// SubscriptionPayment), so the next one comes after it.
	LastPaidDate *time.Time
	// PaidManually is for payments made by hand (rent, bills): a passed
	// payment stays due, overdue, until it's marked paid. Otherwise (a card
	// charged automatically) a passed payment counts as paid.
	PaidManually bool `gorm:"not null;default:false"`
	// IsObligation marks a serious regular payment (rent, insurance, bills)
	// as opposed to a minor subscription (streaming, apps); they're listed
	// and totalled apart.
	IsObligation bool `gorm:"not null;default:false"`
}

// SubscriptionPayment records a payment marked paid: PaidFor is the
// payment's day. Deleting records rolls the schedule back.
type SubscriptionPayment struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	SubscriptionID uint      `gorm:"index;not null"`
	PaidFor        time.Time `gorm:"not null"`
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
