# Payment flags & AML large-tx tagging (#121)

GigaWallet supports free-form **flags** on outgoing payments for compliance workflows.

## Config

```toml
[Gigawallet]
# FinCEN-style optional threshold in USD (0 = disabled)
AMLThresholdUSD = 10000
# Static DOGE/USD rate used for conversion (keep updated)
DogeUSDPrice = 0.15
```

When both are positive, any outgoing payment whose  
`total_doge * DogeUSDPrice >= AMLThresholdUSD` is tagged with `"AML"`.

## Explicit flags

```http
POST /account/:foreignID/pay
{ "amount": "50000", "to": "D…", "flags": ["AML", "MANUAL_REVIEW"] }
```

## Review API

```http
GET /account/:foreignID/payments/flagged?flag=AML&cursor=0&limit=50
```

Returns `{ items, cursor, flag }`. Items include the `flags` array.

## Events

- `PAYMENT_SENT` includes `flags` when present  
- `PAYMENT_FLAGGED` is also emitted when flags is non-empty (for webhooks/loggers)

## Notes

- Flags do not block payouts — they **tag and notify** for operator review / SAR workflow.  
- USD conversion is intentionally static/config-driven (no external oracle dependency).  
- Generic flags support other use cases (`["VIP"]`, `["REFUND"]`, …).
