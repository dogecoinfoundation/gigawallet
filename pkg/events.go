package giga

// Gigawallet event types

// bus.Send(INV_PAYMENT_REFUNDED, payment)
// bus.Send(ACC_CREATED, acc)

// Special category, do not use directly, represents *
type EVENT_ALL string

// System Events
type EVENT_SYS string

const (
	SYS_STARTUP EVENT_SYS = "STARTUP"
)

// Network Events
type EVENT_NET string

// Account Events
type EVENT_ACC string

const (
	ACC_CREATED EVENT_ACC = "ACC_CREATED"
	ACC_UPDATED EVENT_ACC = "ACC_UPDATED"
	ACC_PAYMENT EVENT_ACC = "ACC_PAYMENT"
)

// Invoice Events
type EVENT_INV string

const (
	INV_CREATED          EVENT_INV = "INV_CREATED"
	INV_UPDATED          EVENT_INV = "INV_UPDATED"
	INV_PAYMENT_RECEIVED EVENT_INV = "INV_PAYMENT_RECEIVED"
	INV_PAYMENT_VERIFIED EVENT_INV = "INV_PAYMENT_VERIFIED"
	INV_PAYMENT_REFUNDED EVENT_INV = "INV_PAYMENT_REFUNDED"
)
