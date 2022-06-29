![GigaWallet Logo](/doc/gigawallet-logo.png)

The Dogecoin GigaWallet is a backend service which provides a
convenient integration API for platforms such as online shops,
exchanges, social media platforms etc, to transact Dogecoin on
behalf of their users.

The purpose of the GigaWallet is to allow the rapid uptake of 
Dogecoin as a payment option, by taking the complexity and 
risk out of integrating Dogecoin payments. 

## DogeConnect: Payment Protocol

The DogeConnect JSON protocol for authorising Dogecoin transactions 
via interaction with a user's wallet on their device is a key part
of the GigaWallet project. 

The DogeConnect protocol make it possible for any wallet provider
to act as an authorising agent for transactions initiated by
platforms using GigaWallet (or other payment backend implementing 
the protocol.)

[DogeConnect Payment Protocol Specification 0.1](/doc/dogeconnect-spec-0.1.md)

## DogeConnect: SDK

The DogeConnect SDK will provide a reference client for the 
DogeConnect Protocol, and combined with the GigaWallet 
will allow developers to create systems that authorise payments 
from backend to mobile frontend in a non-custodial manner. 


## Major Components / Architecture overview

![Major components of the GigaWallet / DogeConnect Project](/doc/gigawallet-connect.png)
