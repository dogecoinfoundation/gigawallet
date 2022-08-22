package giga

import (
	"time"

	"github.com/shopspring/decimal"
)

// ConnectEnvelope is a wrapper for a ConnectRequest.
// The Payload is a base64 encoded JSON string containing
// a ConnectRequest
type ConnectEnvelope struct {
	Type           string `json:"type"`
	ServiceName    string `json:"service_name"`
	ServiceIcon    string `json:"service_icon"`
	ServiceGateway string `json:"service_gateway"`
	ServiceKey     string `json:"service_key"`
	Payload        []byte `json:"payload"` // the json package will automatically base64 encode and decode this
	Hash           string `json:"hash"`
}

type ConnectRequest struct {
	Type       string          `json:"type"`
	ID         string          `json:"request_id"`
	Address    string          `json:"address"`
	Total      decimal.Decimal `json:"Total"`
	Initiated  time.Time       `json:"initiated"`
	TimeoutSec int             `json:"timeout_sec"`
	Items      []ConnectItem   `json:"items"`
}

type ConnectItem struct {
	Type        string          `json:"type"`
	ID          string          `json:"item_id"`
	Thumb       string          `json:"thumb"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	UnitCount   int             `json:"unit_count"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
}
