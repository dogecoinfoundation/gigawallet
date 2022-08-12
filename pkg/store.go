package giga

type PaymentsStore interface {
	// NewOrder stores an order in the payments store and returns the order ID
	NewOrder(order Order) (string, error)
	// GetOrder returns the order with the given ID
	GetOrder(id string) (Order, error)
}
