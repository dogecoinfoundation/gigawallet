# Multiple payout targets (#113)

GigaWallet already supports **paying multiple Dogecoin addresses in a single transaction**, including splitting the network fee across outputs (e.g. a payment processor’s fee).

## Single destination (simple form)

```http
POST /account/:foreignID/pay
Content-Type: application/json

{
  "amount": "10.0",
  "to": "DPeTgZm7LabnmFTJkAPfADkwiKreEMmzio"
}
```

## Multiple destinations (array form)

```http
POST /account/:foreignID/pay
Content-Type: application/json

{
  "pay": [
    { "amount": "9.0", "to": "DCustomerAddress……………", "deduct_fee_percent": "0" },
    { "amount": "1.0", "to": "DProcessorFeeAddress…………", "deduct_fee_percent": "100" }
  ],
  "max_fee": "1.0"
}
```

Semantics:

| Field | Meaning |
|-------|---------|
| `pay[].amount` | DOGE to that address (before fee deduction rules) |
| `pay[].to` | Destination P2PKH address |
| `pay[].deduct_fee_percent` | Optional share of the **network fee** taken from this output (should sum to ~100 if used) |
| `explicit_fee` | Optional fixed fee override |
| `max_fee` | Cap on calculated fee (default max applies if omitted/zero) |

`deduct_fee_percent` is how a **processing fee** use-case is modeled: the customer output can stay whole while fee comes out of the processor’s output (or split).

## Build-only (don’t broadcast)

```http
POST /account/:foreignID/paytx
```

Same body as `/pay`. Returns the signed transaction hex without broadcasting (`sendTx=false`).

## Notes

- Dust limits apply per output after fee deduction.
- Account must have confirmed UTXOs covering total + fee.
- Internal API: `API.SendFundsToAddress(foreignID, []PayTo{...}, explicitFee, maxFee, sendTx)`.

This document closes the documentation gap for issue #113 — the wire format has been supported via the `pay` array for some time.
