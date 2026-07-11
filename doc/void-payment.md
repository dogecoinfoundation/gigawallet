# Voiding a stuck payment

When `POST /account/:id/pay` builds and reserves UTXOs but **broadcast fails**
(or the process crashes mid-submit), those UTXOs stay reserved (`spend_payment`
set) and cannot fund a retry.

## Recover

```http
POST /account/:foreignID/payment/:paymentID/void
```

Effects:

1. Clears `spend_payment` on UTXOs reserved for that payment  
2. Sets `paid_txid` to the sentinel `VOID`  
3. Refuses if the payment already has a real on-chain txid  

After voiding, create a new payout as usual.

## Related

- Created after failed send: see comments in `API.SendFundsToAddress`  
- Crash during `dogecoin_tx_sign_input` (#110) can leave similar reserved state if the payment row was already created
