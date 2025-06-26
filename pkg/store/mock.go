package store

import (
	"sort"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// NewMock returns a giga.Store that stores orders in memory.
func NewMock() *Mock {
	return &Mock{
		invoices:          make(map[giga.Address]giga.Invoice, 10),
		accounts:          make(map[string]*giga.Account, 10),
		accountsByAddress: make(map[giga.Address]*giga.Account, 10),
		payments:          make(map[int64]giga.Payment, 10),
		paymentOutputs:    make(map[int64][]giga.PayTo, 10),
		nextPaymentID:     1,
	}
}

// Mock

type Mock struct {
	invoices          map[giga.Address]giga.Invoice
	accounts          map[string]*giga.Account
	accountsByAddress map[giga.Address]*giga.Account
	payments          map[int64]giga.Payment
	paymentOutputs    map[int64][]giga.PayTo
	nextPaymentID     int64
}

var _ giga.Store = &Mock{}

func (m *Mock) Begin() (giga.StoreTransaction, error) {
	return &MockTx{store: m}, nil
}

func (m *Mock) GetAccount(foreignID string) (giga.Account, error) {
	v, ok := m.accounts[foreignID]
	if !ok {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %v", foreignID)
	}
	return *v, nil
}

func (m *Mock) CalculateBalance(accountID giga.Address) (giga.AccountBalance, error) {
	return giga.AccountBalance{}, nil
}

func (m *Mock) GetInvoice(id giga.Address) (giga.Invoice, error) {
	v, ok := m.invoices[id]
	if !ok {
		return giga.Invoice{}, giga.NewErr(giga.NotFound, "invoice not found: %v", id)
	}
	return v, nil
}

func (m *Mock) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	return
}

func (m *Mock) GetPayment(account giga.Address, id int64) (giga.Payment, error) {
	payment, ok := m.payments[id]
	if !ok {
		return giga.Payment{}, giga.NewErr(giga.NotFound, "payment not found: %v", id)
	}
	if payment.AccountAddress != account {
		return giga.Payment{}, giga.NewErr(giga.NotFound, "payment not found for account: %v", id)
	}

	// Make a deep copy of the payment
	result := payment

	// Make a deep copy of the PayTo slice
	if payTo, exists := m.paymentOutputs[id]; exists {
		result.PayTo = make([]giga.PayTo, len(payTo))
		copy(result.PayTo, payTo)
	}

	return result, nil
}

func (m *Mock) ListPayments(account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	// Collect all payments for this account (without deep copying yet)
	var accountPayments []giga.Payment
	for _, payment := range m.payments {
		if payment.AccountAddress == account {
			accountPayments = append(accountPayments, payment)
		}
	}

	// Sort by ID using Go's standard library
	sort.Slice(accountPayments, func(i, j int) bool {
		return accountPayments[i].ID < accountPayments[j].ID
	})

	// Apply cursor (skip payments)
	start := int(cursor)
	if start >= len(accountPayments) {
		// No more payments to return
		return []giga.Payment{}, 0, nil
	}

	// Apply limit
	end := start + limit
	if end > len(accountPayments) {
		end = len(accountPayments)
	}

	// Now make deep copies of only the payments we're returning
	items = make([]giga.Payment, end-start)
	for i, payment := range accountPayments[start:end] {
		// Make a deep copy of the payment
		paymentCopy := payment

		// Make a deep copy of the PayTo slice
		if payTo, exists := m.paymentOutputs[payment.ID]; exists {
			paymentCopy.PayTo = make([]giga.PayTo, len(payTo))
			copy(paymentCopy.PayTo, payTo)
		}

		items[i] = paymentCopy
	}

	// Calculate next cursor
	if end < len(accountPayments) {
		next_cursor = int64(end)
	} else {
		next_cursor = 0 // No more payments
	}

	return
}

func (m *Mock) GetAllUnreservedUTXOs(account giga.Address) ([]giga.UTXO, error) {
	return nil, nil
}

func (m *Mock) GetChainState() (giga.ChainState, error) {
	return giga.ChainState{NextSeq: 1}, nil
}

func (m *Mock) GetServiceCursor(name string) (int64, error) {
	return 0, nil
}

func (m *Mock) Close() {}

// MockTx

type MockTx struct {
	store *Mock
}

var _ giga.StoreTransaction = &MockTx{}

func (m MockTx) Commit() error {
	return nil
}

func (m MockTx) Rollback() error {
	return nil
}

func (m MockTx) GetAccount(foreignID string) (giga.Account, error) {
	return m.store.GetAccount(foreignID)
}

func (m MockTx) GetAccountByID(accountID giga.Address) (giga.Account, error) {
	v, ok := m.store.accountsByAddress[accountID]
	if !ok {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %v", accountID)
	}
	return *v, nil
}

func (m MockTx) CalculateBalance(accountID giga.Address) (giga.AccountBalance, error) {
	return m.store.CalculateBalance(accountID)
}

func (m MockTx) UpdateAccountBalance(accountID giga.Address, bal giga.AccountBalance) error {
	return nil
}

func (m MockTx) ListAccountsModifiedSince(cursor int64, limit int) (ids []string, nextCursor int64, err error) {
	return
}

func (m MockTx) GetInvoice(id giga.Address) (giga.Invoice, error) {
	return m.store.GetInvoice(id)
}

func (m MockTx) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	return m.store.ListInvoices(account, cursor, limit)
}

func (m MockTx) GetAllUnreservedUTXOs(account giga.Address) ([]giga.UTXO, error) {
	return m.store.GetAllUnreservedUTXOs(account)
}

func (m MockTx) GetChainState() (giga.ChainState, error) {
	return m.store.GetChainState()
}

func (m MockTx) StoreInvoice(invoice giga.Invoice) error {
	// Check if invoice already exists
	if _, exists := m.store.invoices[invoice.ID]; exists {
		return giga.NewErr(giga.AlreadyExists, "invoice already exists: %v", invoice.ID)
	}

	// Store the invoice
	m.store.invoices[invoice.ID] = invoice
	return nil
}

func (m MockTx) CreateAccount(account giga.Account) error {
	if _, exists := m.store.accounts[account.ForeignID]; exists {
		return giga.NewErr(giga.AlreadyExists, "account already exists: %s", account.ForeignID)
	}

	// Create a single account instance and store pointers to it
	accountPtr := &account
	m.store.accounts[account.ForeignID] = accountPtr
	m.store.accountsByAddress[account.Address] = accountPtr
	return nil
}

func (m MockTx) UpdateAccount(account giga.Account) error {
	if _, exists := m.store.accounts[account.ForeignID]; !exists {
		return giga.NewErr(giga.AlreadyExists, "account does not exist: %s", account.ForeignID)
	}

	// Update the existing account instance
	accountPtr := &account
	m.store.accounts[account.ForeignID] = accountPtr
	m.store.accountsByAddress[account.Address] = accountPtr
	return nil
}

func (m MockTx) StoreAddresses(accountID giga.Address, addresses []giga.Address, firstAddress uint32, internal bool) error {
	return nil
}

func (m MockTx) FindAccountForAddress(pkhAddress giga.Address) (accountID giga.Address, keyIndex uint32, isInternal bool, err error) {
	return
}

func (m MockTx) UpdateChainState(state giga.ChainState, writeRoot bool) error {
	return nil
}

func (m MockTx) CreateUTXO(utxo giga.UTXO) error {
	return nil
}

func (m MockTx) MarkUTXOReserved(txID string, vOut int, paymentID int64) error {
	return nil
}

func (m MockTx) MarkUTXOSpent(txID string, vOut int, spentHeight int64, spendTxID string) (accountId string, scriptAddress giga.Address, err error) {
	return
}

func (m MockTx) MarkPaymentsOnChain(txIDs []string, blockHeight int64) (affectedAcconts []string, err error) {
	return
}

func (m MockTx) RevertChangesAboveHeight(maxValidHeight int64, nextSeq int64) (newSeq int64, err error) {
	return nextSeq, nil
}

func (m MockTx) IncChainSeqForAccounts(accounts map[string]int64) error {
	return nil
}

func (m MockTx) ConfirmPayments(confirmations int, blockHeight int64) (affectedAcconts []string, err error) {
	return
}

func (m MockTx) ConfirmUTXOs(confirmations int, blockHeight int64) (affectedAcconts []string, err error) {
	return
}

func (m MockTx) MarkInvoicesPaid(blockHeight int64, blockID string) (affectedAcconts []string, err error) {
	return
}

func (m MockTx) MarkInvoiceEventSent(invoiceID giga.Address, event giga.EVENT_INV) error {
	return nil
}

func (m MockTx) CreatePayment(account giga.Address, payTo []giga.PayTo, total giga.CoinAmount, fee giga.CoinAmount) (giga.Payment, error) {
	// Generate next payment ID
	paymentID := m.store.nextPaymentID
	m.store.nextPaymentID++

	// Create the payment
	payment := giga.Payment{
		ID:             paymentID,
		AccountAddress: account,
		PayTo:          payTo,
		Total:          total,
		Fee:            fee,
		Created:        time.Now(),
	}

	// Store the payment
	m.store.payments[paymentID] = payment

	// Store the payment outputs (PayTo)
	m.store.paymentOutputs[paymentID] = payTo

	return payment, nil
}

func (m MockTx) GetPayment(account giga.Address, id int64) (giga.Payment, error) {
	return m.store.GetPayment(account, id)
}

func (m MockTx) UpdatePaymentWithTxID(paymentID int64, txID string) error {
	payment, exists := m.store.payments[paymentID]
	if !exists {
		return giga.NewErr(giga.NotFound, "payment not found: %v", paymentID)
	}

	payment.PaidTxID = txID
	m.store.payments[paymentID] = payment
	return nil
}

func (m MockTx) ListPayments(account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	return m.store.ListPayments(account, cursor, limit)
}

func (m MockTx) SetServiceCursor(name string, cursor int64) error {
	return nil
}
