package broker

type BrokerEventType int

const (
	// a new Invoice has been created
	NewInvoice BrokerEventType = iota
	InvoiceInBlock
	InvoiceConfirmed
)

type BrokerEvent struct {
	Type   BrokerEventType
	ID     string // invoice ID
	TxnID  string // transaction ID
	Height int64  // block height
}

type BrokerEmitter interface {
	Subscribe(chan<- BrokerEvent)
}
