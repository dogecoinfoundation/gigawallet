-- SQL Migration from v0.1.0-beta2 to v0.1.1

ALTER TABLE payment DROP COLUMN pay_to;
ALTER TABLE payment DROP COLUMN amount;
ALTER TABLE payment ADD COLUMN total NUMERIC(18,8) DEFAULT 0;

CREATE TABLE IF NOT EXISTS output (
  payment_id INTEGER NOT NULL,
  vout INTEGER NOT NULL,
  pay_to TEXT NOT NULL,
  amount NUMERIC(18,8) NOT NULL,
  deduct_fee_percent NUMERIC(18,8) NOT NULL,
  PRIMARY KEY (payment_id, vout)
);
