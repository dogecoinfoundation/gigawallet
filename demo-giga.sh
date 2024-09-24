#!/usr/bin/env bash

echo Creating account:
curl -X POST http://localhost:8081/account/raffe -d '{}'
echo
echo

echo Getting account:
curl -X GET http://localhost:8081/account/raffe
echo
echo

echo Getting account balance:
curl -X GET http://localhost:8081/account/raffe/balance
echo
echo

echo Creating invoice:
curl -X POST http://localhost:8081/account/raffe/invoice -d '{"vendor":"raffe","items":[{"type":"item","name":"Foods","value":"10","quantity":1}]}'
echo
echo

echo Make a payment to the above 'pay_to_address' and list invoices again.
echo After 6 blocks of confirmation, the invoice will be marked paid.
echo Then, check account balance again - basically re-run this script.
echo

echo Listing invoices:
curl -X GET http://localhost:8081/account/raffe/invoices -s | jq .
echo
echo

INVOICE_ID="DSDAt1e5yeAaqkeVcJHATUpKjHVECCQaPQ"
echo Get an invoice:
curl -X GET "http://localhost:8081/account/raffe/invoice/$INVOICE_ID" -s | jq .
echo
echo
