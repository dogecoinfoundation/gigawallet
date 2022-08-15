package store

import (
	"database/sql"
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

// interface guard ensures SQLite implements giga.PaymentsStore
var _ giga.PaymentsStore = SQLite{}

type SQLite struct {
	db *sql.DB
}

// NewSQLite returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLite(fileName string) (SQLite, error) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		return SQLite{}, err
	}
	return SQLite{db}, nil
}

func (d SQLite) NewOrder(seller giga.Address, order giga.Order) error {
	// TODO: make the sql query
	return nil
}

func (d SQLite) GetOrder(seller giga.Address) (giga.Order, error) {
	// TODO: make the sql query
	return giga.Order{}, nil
}
