package giga

// Gigawallet event types

// bus.Send(INV_PAYMENT_REFUNDED, payment)
// bus.Send(ACC_CREATED, acc)

// Interface for any event
type EventType interface {
	Type() string
}

// slice of all msg types for config funcs lookup
var EVENT_TYPES []EventType = []EventType{EVENT_ALL("ALL"),
	EVENT_SYS("SYS"),
	EVENT_NET("NET"),
	EVENT_ACC("ACC"),
	EVENT_INV("INV")}

// Special category, do not use directly, represents *
type EVENT_ALL string

func (e EVENT_ALL) Type() string {
	return "ALL"
}

// System Events
type EVENT_SYS string

func (e EVENT_SYS) Type() string {
	return "SYS"
}

const (
	SYS_STARTUP EVENT_SYS = "SYS_STARTUP"
	SYS_ERR     EVENT_SYS = "SYS_ERR"
	SYS_MSG     EVENT_SYS = "SYS_MSG"
)

// Network Events
type EVENT_NET string

func (e EVENT_NET) Type() string {
	return "NET"
}

// Account Events
type EVENT_ACC string

func (e EVENT_ACC) Type() string {
	return "ACC"
}

const (
	ACC_CREATED        EVENT_ACC = "ACC_CREATED"
	ACC_UPDATED        EVENT_ACC = "ACC_UPDATED"
	ACC_PAYMENT        EVENT_ACC = "ACC_PAYMENT"
	ACC_CHAIN_ACTIVITY EVENT_ACC = "ACC_CHAIN_ACTIVITY"
	ACC_BALANCE_CHANGE EVENT_ACC = "ACC_BALANCE_CHANGE"
)

type AccPaymentSentEvent struct {
	From   string     `json:"from"`
	PayTo  Address    `json:"pay_to"`
	Amount CoinAmount `json:"amount"`
	TxID   string     `json:"txid"`
}

type AccBalanceChangeEvent struct {
	AccountID       Address    `json:"account_id"`
	ForeignID       string     `json:"foreign_id"`
	CurrentBalance  CoinAmount `json:"current_balance"`
	IncomingBalance CoinAmount `json:"incoming_balance"`
	OutgoingBalance CoinAmount `json:"outgoing_balance"`
}

// Invoice Events
type EVENT_INV string

func (e EVENT_INV) Type() string {
	return "INV"
}

const (
	INV_CREATED                 EVENT_INV = "INV_CREATED"
	INV_PAYMENT_SENT            EVENT_INV = "INV_PAYMENT_SENT"
	INV_PART_PAYMENT_DETECTED   EVENT_INV = "INV_PART_PAYMENT_DETECTED"
	INV_TOTAL_PAYMENT_DETECTED  EVENT_INV = "INV_TOTAL_PAYMENT_DETECTED"
	INV_OVER_PAYMENT_DETECTED   EVENT_INV = "INV_OVER_PAYMENT_DETECTED"
	INV_TOTAL_PAYMENT_CONFIRMED EVENT_INV = "INV_TOTAL_PAYMENT_CONFIRMED"
	INV_OVER_PAYMENT_CONFIRMED  EVENT_INV = "INV_OVER_PAYMENT_CONFIRMED"
	INV_PAYMENT_UNCONFIRMED     EVENT_INV = "INV_PAYMENT_UNCONFIRMED"
	INV_PAYMENT_REFUNDED        EVENT_INV = "INV_PAYMENT_REFUNDED"
)

type InvPaymentEvent struct {
	InvoiceID      Address    `json:"invoice_id"`
	AccountID      Address    `json:"account_id"`
	ForeignID      string     `json:"foreign_id"`
	InvoiceTotal   CoinAmount `json:"invoice_total"`
	TotalIncoming  CoinAmount `json:"total_incoming"`
	TotalConfirmed CoinAmount `json:"total_confirmed"`
}

type InvOverpaymentEvent struct {
	InvoiceID            Address    `json:"invoice_id"`
	AccountID            Address    `json:"account_id"`
	ForeignID            string     `json:"foreign_id"`
	InvoiceTotal         CoinAmount `json:"invoice_total"`
	TotalIncoming        CoinAmount `json:"total_incoming"`
	TotalConfirmed       CoinAmount `json:"total_confirmed"`
	OverpaymentIncoming  CoinAmount `json:"overpayment_incoming"`
	OverpaymentConfirmed CoinAmount `json:"overpayment_confirmed"`
}
