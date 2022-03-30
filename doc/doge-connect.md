# DogeConnect

DogeConnect is an open protocol for initiating payment requests to non-custodial
Dogecoin wallets. It is primarily intended to provide a mechanism for merchants to
interact with people wanting to spend Dogecoin as a payment for products and 
services.

DogeConnect protocol is a series of JSON objects using Dogecoin pub/priv keys to
sign and HTTP/REST APIs to manage payment flow. 

**Major type definitions:**

## DogeConnect Envelope

```js
{
  type: "dc:0.1:envelope",
  service: "My Service Name", 
  service_icon: "https://example.com/icon.png",
  service_gateway: "https://example.com/payments",
  service_key: "6JWBrzyPQnvZnob7JVvq85PYcCt8wqd6ksZyfTVakMS6iYSAUuC",
  payload: "base64 encoded JSON"
  hash:  "payload hash (from service_key)"
}
```

#### type

* **type**: string
* **required**: true
 
Type is a descriminator that can be used to identify a JSON structure, it is a colon-separated
string made of three parts:

* 'dc' indicates this is a DogeConnect data structure.
* '0.1' indicates the protocol version.
* 'envelope' indicates the DogeConnect type.

#### service

* **type**: string
* required: true

Service provides a name which the UI can use to identify the entity requesting payment. This 
should be a short, easily identifiable string that the user would recognise as the originator
of a payment request for the app/store they are interacting with, ie: "AMC Theatres", 
"Tesla Merchansise" etc

#### service_icon

* **type**: string
* required: false

Service_icon is an optional https accessible image, intended for use as an icon/logo that a
user-agent can display along with the Service name. Icons should be 1:1 (square) aspect ratio
and may be cached by the user-agent.

#### service_gateway

* **type**: string
* **required**: true

Service_gateway is a required https URL that provides the BASE path for a DogeConnect Gateway
such as GigaWallet. As a bare minimum a DogeConnect Gateway MUST provide:

* POST <base path>/pay   -> 200 OK    for accepting signed payment rersponses

#### service_key

* **type**: string
* **required**: true

Service_key is a public key (dogecoin address) that the DogeConnect Gateway holds a private key 
to, which it uses to sign outgoing payment request envelopes. The `service_key` is used by the 
user-agent to validate the `payload` via the `hash` in the envelope. This service_key MUST be 
treated by the user-agent as an immutable identifier for that service. 

It is expected that a user-agent will, on recieving a request for payment from a new service,
allow the user to accept (or reject) the new service. Any future payments from that service 
would be expected to use the same service_key, a change of service_key should be treated as 
a new service and be presented to the user as such, with appropriate warnings.

Note: This is understood to provide limited security without further infrastructure to allow
a service to list their service_key somewhere else for validation. We expect this to be solved
in the future by other means out of scope for this current protocol. 

#### payload

* **type**: string
* **required**: true

The payload is a base64 encoded string containing the JSON data for the internal DogeConnect 
payment_request. This base64 string is verifiable via the `hash` and `service_key` fields which
accompany it as part of the payment envelope.


##### hash

* **type**: string
* **required**: true

The hash verifies the content of the `payload` was signed with the private key component of 
the public `service_key`. The hash function is the same provided by libdogecoin for signing
transactions.



## Doge Connect Payload

The Doge Connect Envelope privides a verifiable mechanism for transmitting a payment request
to a user-agent. The payload can take the form of a number of payment options:


## Payment Request

```js
{
  type: "dc:0.1:payment_request",
  request_id: "8TSBrzyPQnvZnob7JVvq85PYcCt8wqd6ksZyfTVakMS6iYSAVu9",
  address: "wqd6ksZyfTVakMS6iYSAVu98TSBrzyPQnvZnob7JVvq85PYcCt8",
  amount: 123.45,
  initiated: "2022-03-29T22:18:26.625Z",
  timeout_sec: 900,
  items: [
    {
      type: "dc:0.1:payment_item",
      item_id: "123456",
      item_thumb: "https://example.com/123456/thumb.png",
      name: "Example Item",
      description: "A rather nice Example Item",
      unit_count: 1,
      unit_amount: "50.0",
      amount: "50.0"
    }, 
    ...
  ]
  
}
```

Docs TBD

## Donation Request

```js
{
  type: "dc:0.1:donation_request",
  request_id: "8TSBrzyPQnvZnob7JVvq85PYcCt8wqd6ksZyfTVakMS6iYSAVu9",
  address: "wqd6ksZyfTVakMS6iYSAVu98TSBrzyPQnvZnob7JVvq85PYcCt8",
  initiated: "2022-03-29T22:18:26.625Z",
  modes: ["OPTIONS_SINGLE", "ANY", "COMMENT"],
  options : [
    {
      type: "dc:0.1:donation_option",
      option_id: "123456",
      item_thumb: "https://example.com/123456/thumb.png",
      name: "High Roller",
      description: "Your donation will support Example Org for 3 months",
      amount: "5000000.0"
    }, 
    {
      type: "dc:0.1:donation_option",
      option_id: "456789",
      item_thumb: "https://example.com/456789/thumb.png",
      name: "bread'n'Butter Backer",
      description: "Your donation contributes to Example Cause",
      amount: "5000.0"
    }, 
    ...
  ]
  
}
```

Docs TBD


