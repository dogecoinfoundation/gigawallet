package store

import (
	"errors"
	"fmt"
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

// interface guard ensures Mock implements giga.PaymentsStore
var _ giga.PaymentsStore = Mock{}

type Mock struct {
	orders map[giga.Address]giga.Order
}

// NewMock returns a giga.PaymentsStore implementor that stores orders in memory
func NewMock() Mock {
	return Mock{}
}

func (d Mock) NewOrder(seller giga.Address, order giga.Order) error {
	d.orders[seller] = order
	return nil
}

func (d Mock) GetOrder(seller giga.Address) (giga.Order, error) {
	v, ok := d.orders[seller]
	if !ok {
		return giga.Order{}, errors.New("no order under address " + fmt.Sprint(seller))
	}
	return v, nil
}
