package giga

// L1 represents access to Dogecoin's L1 functionality.
//
// The general idea is that this will eventually be provided by a
// Go binding for the libdogecoin project, however to begin with
// will be implemented via RPC/ZMQ comms to the Dogecoin Core APIs.
type L1 interface {
	MakeAddress() (Address, error)
	Send(Txn) error
}

type Address struct {
	PrivKey string
	PubKey  string
}

type Txn struct{}
