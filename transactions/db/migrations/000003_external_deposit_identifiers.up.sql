-- 000003_external_deposit_identifiers.up.sql
-- Changes the deposit identifier columns from UUID-specific storage with
-- relational constraints to external string identifiers. For the HighLevel
-- payment flow:
--   client_name  = "highlevel-<locationId>" (RVPay client-name convention)
--   customer_id  = HighLevel contact.id (optional)
--   merchant_id  = HighLevel transactionId (optional, temporary mapping)
-- Existing UUID values are preserved as their canonical text form.

ALTER TABLE deposits
    DROP CONSTRAINT IF EXISTS fk_deposit_customer,
    DROP CONSTRAINT IF EXISTS fk_deposit_merchant;

DROP INDEX IF EXISTS idx_deposits_client_id;
DROP INDEX IF EXISTS idx_deposits_customer_id;
DROP INDEX IF EXISTS idx_deposits_merchant_id;

ALTER TABLE deposits
    RENAME COLUMN client_id TO client_name;

ALTER TABLE deposits
    ALTER COLUMN client_name TYPE TEXT USING client_name::TEXT,
    ALTER COLUMN customer_id TYPE TEXT USING customer_id::TEXT,
    ALTER COLUMN customer_id DROP NOT NULL,
    ALTER COLUMN merchant_id TYPE TEXT USING merchant_id::TEXT,
    ALTER COLUMN merchant_id DROP NOT NULL;

CREATE INDEX idx_deposits_client_name ON deposits (client_name);

CREATE INDEX idx_deposits_customer_id ON deposits (customer_id);

CREATE INDEX idx_deposits_merchant_id ON deposits (merchant_id);