package test

import (
	"testing"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
)

const address1 = "D9YL12TaaLJKuUe2aYoGKGAuDnU9RN22Mc"
const iconURL = "https://example.com/icon"
const streetAddr = "123 Street"

func TestAPI(t *testing.T) {
	bus := giga.NewMessageBus()
	conf := giga.Config{}
	follower := FollowerMock{}
	mockStore := store.NewMock()
	l1, _ := dogecoin.NewL1Mock(conf)

	api := giga.NewAPI(mockStore, l1, bus, follower, conf)

	// create new account
	acc, err := api.CreateAccount(map[string]any{
		"payout_address":   address1,
		"payout_threshold": "12.83",
		"payout_frequency": "xyzzy",
		"vendor_name":      "Vendredi",
		"vendor_icon":      iconURL,
		"vendor_address":   "123 Street",
	}, "xyz", true)
	if err != nil {
		t.Fatalf("Create Account: %v", err)
	}
	if acc.PayoutAddress != address1 {
		t.Fatalf("Create Account: wrong payout address: %v vs %v", acc.PayoutAddress, address1)
	}
	if acc.PayoutThreshold.String() != "12.83" {
		t.Fatalf("Create Account: wrong payout threshold: %v vs %v", acc.PayoutAddress, "12.83")
	}
	if acc.PayoutFrequency != "xyzzy" {
		t.Fatalf("Create Account: wrong payout frequency: %v vs %v", acc.PayoutAddress, "xyzzy")
	}
	if acc.VendorName != "Vendredi" {
		t.Fatalf("Create Account: wrong vendor name: %v vs %v", acc.PayoutAddress, "Vendredi")
	}
	if acc.VendorIcon != iconURL {
		t.Fatalf("Create Account: wrong vendor icon: %v vs %v", acc.PayoutAddress, iconURL)
	}
	if acc.VendorAddress != streetAddr {
		t.Fatalf("Create Account: wrong vendor address: %v vs %v", acc.PayoutAddress, streetAddr)
	}

	// check the stored account
	acc, err = api.GetAccount("xyz")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.PayoutAddress != address1 {
		t.Fatalf("Get Account: wrong payout address: %v vs %v", acc.PayoutAddress, address1)
	}
	if acc.PayoutThreshold.String() != "12.83" {
		t.Fatalf("Get Account: wrong payout threshold: %v vs %v", acc.PayoutAddress, "12.83")
	}
	if acc.PayoutFrequency != "xyzzy" {
		t.Fatalf("Get Account: wrong payout frequency: %v vs %v", acc.PayoutAddress, "xyzzy")
	}
	if acc.VendorName != "Vendredi" {
		t.Fatalf("Get Account: wrong vendor name: %v vs %v", acc.PayoutAddress, "Vendredi")
	}
	if acc.VendorIcon != iconURL {
		t.Fatalf("Get Account: wrong vendor icon: %v vs %v", acc.PayoutAddress, iconURL)
	}
	if acc.VendorAddress != streetAddr {
		t.Fatalf("Get Account: wrong vendor address: %v vs %v", acc.PayoutAddress, streetAddr)
	}

	// try an invalid field name
	_, err = api.CreateAccount(map[string]any{
		"bad_field": "OK",
	}, "abc", true)
	if err == nil {
		t.Fatalf("Create Account: should report unknown setting: bad_field")
	}

	// partially update existing account
	acc, err = api.CreateAccount(map[string]any{
		"payout_threshold": "11.95",
		"vendor_name":      "Vendredo",
	}, "xyz", true)
	if err != nil {
		t.Fatalf("Update Account: %v", err)
	}
	if acc.PayoutAddress != address1 {
		t.Fatalf("Update Account: wrong payout address: %v vs %v", acc.PayoutAddress, address1)
	}
	if acc.PayoutThreshold.String() != "11.95" {
		t.Fatalf("Update Account: wrong payout threshold: %v vs %v", acc.PayoutAddress, "11.95")
	}
	if acc.PayoutFrequency != "xyzzy" {
		t.Fatalf("Update Account: wrong payout frequency: %v vs %v", acc.PayoutAddress, "xyzzy")
	}
	if acc.VendorName != "Vendredo" {
		t.Fatalf("Update Account: wrong vendor name: %v vs %v", acc.PayoutAddress, "Vendredo")
	}
	if acc.VendorIcon != iconURL {
		t.Fatalf("Update Account: wrong vendor icon: %v vs %v", acc.PayoutAddress, iconURL)
	}
	if acc.VendorAddress != streetAddr {
		t.Fatalf("Update Account: wrong vendor address: %v vs %v", acc.PayoutAddress, streetAddr)
	}

	// check the updated account
	acc, err = api.GetAccount("xyz")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.PayoutAddress != address1 {
		t.Fatalf("Get Account: wrong payout address: %v vs %v", acc.PayoutAddress, address1)
	}
	if acc.PayoutThreshold.String() != "12.83" {
		t.Fatalf("Get Account: wrong payout threshold: %v vs %v", acc.PayoutAddress, "11.95")
	}
	if acc.PayoutFrequency != "xyzzy" {
		t.Fatalf("Get Account: wrong payout frequency: %v vs %v", acc.PayoutAddress, "xyzzy")
	}
	if acc.VendorName != "Vendredi" {
		t.Fatalf("Get Account: wrong vendor name: %v vs %v", acc.PayoutAddress, "Vendredo")
	}
	if acc.VendorIcon != iconURL {
		t.Fatalf("Get Account: wrong vendor icon: %v vs %v", acc.PayoutAddress, iconURL)
	}
	if acc.VendorAddress != streetAddr {
		t.Fatalf("Get Account: wrong vendor address: %v vs %v", acc.PayoutAddress, streetAddr)
	}
}

type FollowerMock struct{}

func (f FollowerMock) SendCommand(cmd any) {}
