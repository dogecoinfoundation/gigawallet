![Conductor Logo](/docs/logo.png)

The Dogecoin GigaWallet is a backend service which provides a
convenient integration API for platforms such as online shops,
exchanges, social media platforms etc, to transact Dogecoin on
behalf of their users in a non-custodial manner.

The purpose of the GigaWallet is to allow the rapid uptake of 
Dogecoin as a payment option, by taking the complexity and 
risk out of integrating Dogecoin payments. 

## Dogecoin Payment Protocol

The Dogecoin GigaWallet provides a simple JSON protocol
for authorising Dogecoin transactions via interaction with a
user's wallet on their device. By documenting a basic payment
protocol we intend to make it possible for any wallet provider
to act as an authorising agent for transactions initiated by
platforms using GigaWallet.

## Dogecoin Keyring (APP) & SDK

The Dogecoin Keyring App will provide a basic reference client
for the Dogecoin Payment Protocol, making use of the GigaWallet 
SDK to authorise payments from GigaWallet-using platforms with
a rock solid, easy to use HD wallet. This should allow platforms
to get up and running building apps that use the GigaWallet service
and over time provide an example for 3rd party Dogecoin Wallets 
to hook the payments SDK.


## Components

* Config Loader
* L1 Interface
  * Dogecoin Core adaptor
* WalletStore Interface
  * SqliteWalletStore
  * PostgresWalletStore
* Event Bus
  * Event Trigger:  MQTT
  * Event Trigger: Webhooks
* DPP Types
  * Payment Request
  * Transaction
  * ??
* API layer


