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
	SYS_STARTUP EVENT_SYS = "STARTUP"
	SYS_ERR     EVENT_SYS = "ERR"
	SYS_MSG     EVENT_SYS = "MSG"
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
	ACC_CREATED        EVENT_ACC = "CREATED"
	ACC_UPDATED        EVENT_ACC = "UPDATED"
	ACC_PAYMENT        EVENT_ACC = "PAYMENT"
	ACC_CHAIN_ACTIVITY EVENT_ACC = "CHAIN_ACTIVITY"
)

// Invoice Events
type EVENT_INV string

func (e EVENT_INV) Type() string {
	return "INV"
}

const (
	INV_CREATED          EVENT_INV = "CREATED"
	INV_UPDATED          EVENT_INV = "UPDATED"
	INV_PAYMENT_RECEIVED EVENT_INV = "PAYMENT_RECEIVED"
	INV_PAYMENT_SENT     EVENT_INV = "PAYMENT_SENT"
	INV_PAYMENT_VERIFIED EVENT_INV = "PAYMENT_VERIFIED"
	INV_PAYMENT_REFUNDED EVENT_INV = "PAYMENT_REFUNDED"
)
