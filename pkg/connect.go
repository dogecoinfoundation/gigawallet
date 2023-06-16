package giga

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// ConnectEnvelope is a wrapper for a ConnectRequest.
// The Payload is a base64 encoded JSON string containing
// a ConnectRequest
type ConnectEnvelope struct {
	Type           string `json:"type"`
	ServiceName    string `json:"service_name"`
	ServiceIconURL string `json:"service_icon_url"`
	ServiceDomain  string `json:"service_domain"`
	ServiceKeyHash string `json:"service_key_hash"`
	Payload        string `json:"payload"` // the json package will automatically base64 encode and decode this
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

// returns a ConnectEnvelope with a signed ConnectRequest
// for the given Invoice. k is the service private key for
// this gigawallet and should match the public key available
// in DNS.
func InvoiceToConnectRequestEnvelope(i Invoice, k string) (ConnectEnvelope, error) {

	// build a connect request
	r := ConnectRequest{}
	r.Type = "dc:0.1:payment_request"
	r.ID = string(i.ID)
	r.Address = string(i.ID)
	r.Total = i.CalcTotal()
	r.Initiated = time.Now()
	r.TimeoutSec = 60 * 30 // TODO should come from the invoice
	r.Items = []ConnectItem{}

	for _, item := range i.Items {
		r.Items = append(r.Items, ConnectItem{"dc:0.1:payment_item",
			"id",
			item.ImageLink,
			item.Name,
			"Description",
			item.Quantity,
			item.Price,
		})
	}
	// serialise to JSON then base64 the request

	payloadJson, _ := json.Marshal(r)
	payload := base64.StdEncoding.EncodeToString(payloadJson)

	// sign the request with the service key

	// build a connect envelope
	env := ConnectEnvelope{}
	env.Payload = payload

	return env, nil

}
