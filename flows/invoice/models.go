package invoice

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type Invoice struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Title         string
	IsIncoming    bool
	Currency      string
	AmountCents   int64
	Paid          bool
	Peer          string
	InvoiceDate   *time.Time
	DueDate       *time.Time
	TargetAccount string
	URL           string
	Description   string
}

const (
	InvoiceFieldTitle = iota
	InvoiceFieldType
	InvoiceFieldCurrency
	InvoiceFieldAmount
	InvoiceFieldPaid
	InvoiceFieldPeer
	InvoiceFieldInvoiceDate
	InvoiceFieldDueDate
	InvoiceFieldTargetAccountChoice
	InvoiceFieldTargetAccountName
	InvoiceFieldURL
	InvoiceFieldDescription
	InvoiceFieldCount
)

type AddInvoiceForm struct {
	Inputs          []textinput.Model
	Active          int
	CurrencyOptions []string
	CurrencyIndex   int
	AccountOptions  []string
	AccountIndex    int
	IsIncoming      bool
	Paid            bool
}

type EditInvoiceForm struct {
	TitleInput         textinput.Model
	AmountInput        textinput.Model
	PeerInput          textinput.Model
	InvoiceDateInput   textinput.Model
	DueDateInput       textinput.Model
	TargetAccountInput textinput.Model
	URLInput           textinput.Model
	DescriptionInput   textinput.Model
	ActiveField        int
	CurrencyOptions    []string
	CurrencyIndex      int
	AccountOptions     []string
	AccountIndex       int
	IsIncoming         bool
	Paid               bool
}
