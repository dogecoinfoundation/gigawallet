-- SQL Migration from v0.1.0-beta2 to v0.1.1

-- total is the sum of all payment outputs.
-- since all existing payments had a single output, we can rename the old amount column.
ALTER TABLE payment RENAME COLUMN amount TO total;

-- we now store the fee paid by our transactions.
-- since we can't infer this from the data we have, set existing rows to zero.
ALTER TABLE payment ADD COLUMN fee NUMERIC(18,8) DEFAULT 0;
ALTER TABLE payment ALTER COLUMN fee SET NOT NULL;

-- create a table to hold multiple outputs for each payment.
CREATE TABLE IF NOT EXISTS output (
  payment_id INTEGER NOT NULL,
  vout INTEGER NOT NULL,
  pay_to TEXT NOT NULL,
  amount NUMERIC(18,8) NOT NULL,
  deduct_fee_percent NUMERIC(18,8) NOT NULL,
  PRIMARY KEY (payment_id, vout)
);

-- copy existing pay_to and amount (now total) into the new outputs table.
INSERT INTO output (payment_id, vout, pay_to, amount, deduct_fee_percent)
SELECT payment_id, 0 as vout, pay_to, total as amount, 0 as deduct_fee_percent FROM payment;

-- pay_to (and amount) were moved from payment table to output table.
-- these fields no longer exist on the payment table.
ALTER TABLE payment DROP COLUMN pay_to;
