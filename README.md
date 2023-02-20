![GigaWallet Logo](/doc/gigawallet-logo.png)


Dogecoin GigaWallet is a backend service which provides a
convenient integration API for platforms such as online shops,
exchanges, social media platforms etc, to transact Dogecoin on
behalf of their users.

The purpose of the GigaWallet is to promote the rapid uptake of 
Dogecoin as a payment option, by taking the complexity and 
risk out of integrating Dogecoin payments into business. 

You can find what we're currently working on in [TODOs](/TODOs.md)

## DogeConnect: Payment Protocol

The DogeConnect JSON protocol for authorising Dogecoin transactions 
via interaction with a user's wallet on their device is a key part
of the GigaWallet project. 

The DogeConnect protocol make it possible for any self-custodial wallet
to act as an authorising agent for transactions initiated by
platforms using GigaWallet (or other payment backend implementing 
the protocol.) 

This is an extension to BIP 70 payment URLs that provides much more
structured information about a 'cart' or items being purchased, as 
well as protocol for how to send a signed txn back via the vendor.

[DogeConnect Payment Protocol Specification 0.1](/doc/dogeconnect-spec-0.1.md)


## Major Components / Architecture overview

![Major components of the GigaWallet / DogeConnect Project](/doc/gigawallet-connect.png)
