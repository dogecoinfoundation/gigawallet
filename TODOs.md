## What's happening atm? 


#### Cleanup APIs - @tjstebbing 
  - [ ] Move to a sub-package `webapi.go` getting big
  - [ ] Refresh old PR and apply to private endpoints #17
  - [ ] Add invoice reading endpoint with QRcode / JSON (connect) 

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

