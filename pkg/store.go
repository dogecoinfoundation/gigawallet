package giga

type PaymentsStore interface {
	// NewOrder stores an order in the payments store under the given address.
	// The Address should be a one-time use address.
	NewOrder(seller Address, order Order) error
	// GetOrder returns the order under the given address.
	GetOrder(seller Address) (Order, error)
}
