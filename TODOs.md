## What's happening atm? 


#### Cleanup APIs - @tjstebbing 
  - [ ] Refresh old PR and apply to private endpoints #17
  - [ ] Add appropriate JSON (connect) output to invoice api
  - [x] Add invoice reading endpoint with QRcode 
  - [x] Move to a sub-package `webapi.go` getting big

  
#### Auto-reconciliation feature - @tjstebbing
  - [ ] Add account 'settings' to DB
  - [ ] Add reconciliation setting per account (where/when to auto send Doge payments)
  - [ ] Add reconciliation setting per invoice (overrides account setting)
  - [ ] Write an Accountant service to manage automatic payments etc.
  
#### Message broker - @tjstebbing
  - [ ] Build AMQP connector for external event integration
  - [x] Build logger connector for debugging msg bus
  - [x] Build message bus system

#### Transaction Broker - @raffecat
  - [ ] Detect tip reorgs (what happens?) 
  - [x] Connect to core ZMQ channels and recieve blocks

#### Wallet logic - @raffecat
  - [ ] Implement balance querying API (total vs available?)
  - [x] Implement unspent UTXO tracking for payments

