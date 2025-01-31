
# ChainFollower

ChainFollower follows the blockchain by requesting consecutive blocks from
a Dogecoin Core node, decoding those blocks, and making database changes
based on the transactions in the block.

Each time a new block is received (or processed during catch-up or initial sync)
ChainFollower goes through the following steps, in the order listed below.

All of this work for a block is wrapped in a database transaction, so either
the entire block is processed successfully, making all changes listed below,
or the DB transaction rolls back and no changes are made.


## CreateUTXO

Each time a block is received, we insert new UTXOs for all transaction outputs.

How it works: we use a database index (account_address) to find the Account that
owns the paid-to address for all P2PKH outputs; we insert a new UTXO associated
with that account and set the `added_height` to the current block-height.
This puts the UTXO into the `added` state.

We only keep UTXOs for Gigawallet Accounts. Our accounts are HD Wallets.
The account logic keeps the next 20 unused addresses in the database to *catch*
transactions initiated by third-party wallets outside of Gigawallet.
This is implemented in account.go in UpdatePoolAddresses.


## MarkUTXOSpent

Each time a block is received, we move some UTXOs from `spendable` to `spending` state.

How it works: for all transaction inputs in the block that also exist in our DB,
we set the `spending_height` to the current block-height. We also record the spending
transaction id in `spend_txid`.

Note that many of the spent UTXOs in a given block won't exist in our database,
because we only keep UTXOs associated with our Gigawallet accounts' addresses.


## ConfirmUTXOs

Each time a block is received, we move some UTXOs from `added` to `spendable` state.

How it works: we set the `spendable_height` to the current block-height in all UTXOs
with an `added_height` but no `spendable_height`, where the UTXO has N confirmations,
i.e. `current-block-height >= added_height + N` (N is the number of *confirmations*
required for the transaction.)

Each time a block is received, we move some UTXOs from `spending` to `spent` state.

How it works: we set the `spent_height` to the current block-height in all UTXOs
with a `spending_height` but no `spent_height`, where the UTXO has N confirmations,
i.e. `current-block-height >= spending_height + N`


## MarkInvoicesPaid

Each time a block is received, we move some Invoices from `unpaid` to `paid` state.

How it works: we set the `paid_height` to the current block-height in all Invoices
with confirmed UTXOs that pay-to the Invoice Address, and sum up to at least the
Invoice total. Invoices can be over-paid, but that doesn't affect marking them paid.


## MarkPaymentsOnChain

Each time a block is received, we move some Payments from `accepted` to `on-chain` state.

How it works: we scan the DB for all Payments matching the transaction ids in the
new block, setting their `paid_height` to the current block-height.


## ConfirmPayments

Each time a block is received, we move some Payments from `on-chain` to `paid` state.

How it works: we set the `confirmed_height` to the current block-height in all
Payments with a `paid_height` but no `confirmed_height`, where the Payment has N
confirmations, i.e. `current-block-height >= paid_height + N`

How it works: we set the `confirmed_height` to the current block-height in all
Payments with confirmed UTXOs that pay-to the Invoice Address, and sum up to at least the
Invoice total. Invoices can be over-paid, but that doesn't affect marking them paid.


# API Requests

The following behaviour is transactional, occuring only when API requests are made
to Gigawallet.


## CreatePayment

When a Payment is created via the REST API, we create a Payment in `pending` state.

### GetAllUnreservedUTXOs

Gigawallet first finds all UTXOs associated with the user's Account that have been
confirmed (are `spendable`), and have not already been spent or reserved.

From these, Gigawallet selects the oldest confirmed UTXOs first, followed by
unconfirmed *change* UTXOs, to make up the total amount being paid plus fees.
Gigawallet will not spend unconfirmed UTXOs from external parties, i.e. it
only spends its own unconfirmed change, with lowest priority.

These selected UTXOs are used to create and sign a Dogecoin transaction
using `libdogecoin`.

Gigawallet uses Dogecoin Core `estimatefee` RPC to calculate fees.

### MarkUTXOReserved

Gigawallet puts the selected UTXOs into the `reserved` state by setting `spend_payment`
to the ID of the new Payment. This prevents the spent UTXOs being re-used in another
Payment before they appear on-chain in a block, which will trigger the `MarkUTXOSpent`
logic above.

### UpdatePaymentWithTxID

Gigawallet submits the signed Payment transaction to Dogecoin Core, then puts the
Payment into `accepted` state by setting `paid_txid` to the transaction id.
