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
	Version        string `json:"version"`
	ServiceName    string `json:"service_name"`
	ServiceIconURL string `json:"service_icon_url"`
	ServiceDomain  string `json:"service_domain"`
	ServiceKeyHash string `json:"service_key_hash"`
	Payload        string `json:"payload"` // the json package will automatically base64 encode and decode this
	Hash           string `json:"hash"`
}

// A payload within an envelope that represents an invoice for
// a list of items that need to be paid
type ConnectInvoice struct {
	Type       string          `json:"type"` // invoice
	ID         string          `json:"request_id"`
	Address    string          `json:"address"`
	Total      decimal.Decimal `json:"Total"`
	Initiated  time.Time       `json:"initiated"`
	TimeoutSec int             `json:"timeout_sec"`
	Items      []ConnectItem   `json:"items"`
}

// an item within an invoice
type ConnectItem struct {
	Type        string          `json:"type"`
	ID          string          `json:"item_id"`
	Thumb       string          `json:"thumb"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	UnitCount   int             `json:"unit_count"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
}

func InvoiceToConnectRequestEnvelope(i Invoice, conf Config) (ConnectEnvelope, error) {

	// build a connect Invoice
	r := ConnectInvoice{
		Type:       "invoice",
		ID:         string(i.ID),
		Address:    string(i.ID),
		Total:      i.CalcTotal(),
		Initiated:  time.Now(),
		TimeoutSec: 60 * 30, // TODO should come from the invoice
		Items:      []ConnectItem{},
	}

	for _, item := range i.Items {
		r.Items = append(r.Items, ConnectItem{
			Type:        "item",
			ID:          "TODO",
			Thumb:       item.ImageLink,
			Name:        item.Name,
			Description: "Description",
			UnitCount:   item.Quantity,
			UnitCost:    item.Price,
		})
	}
	// serialise to JSON then base64 the request

	payloadJson, _ := json.Marshal(r)
	payload := base64.StdEncoding.EncodeToString(payloadJson)

	// sign the request with the service key
	hash := "TODO"

	// build a connect envelope
	env := ConnectEnvelope{
		Version:        "0.1",
		ServiceName:    conf.Gigawallet.ServiceName,
		ServiceIconURL: conf.Gigawallet.ServiceIconURL,
		ServiceDomain:  conf.Gigawallet.ServiceDomain,
		ServiceKeyHash: conf.Gigawallet.ServiceKeyHash,
		Payload:        payload,
		Hash:           hash,
	}

	return env, nil
}
