package giga

type NodeEventType int

const (
	TX NodeEventType = iota
	Block
)

type NodeEvent struct {
	Type NodeEventType
	ID   string
	Data string
}

type NodeEmitter interface {
	Subscribe(chan<- NodeEvent)
}
