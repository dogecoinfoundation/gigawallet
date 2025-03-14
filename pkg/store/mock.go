package store

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

// NewMock returns a giga.Store implementor that stores orders in memory
func NewMock() giga.Store {
	mock, err := NewSQLiteStore(":memory:")
	if err != nil {
		panic(err)
	}
	return mock
}
