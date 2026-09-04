-- 000004_customer_client_name.down.sql
-- Reverses 000004_customer_client_name. The client_name -> client_id
-- conversion is only valid while every stored client_name value is the
-- canonical text form of the original UUID (same caveat as migration
-- 000003's down). Rows created with external client names after the up
-- migration cannot be converted back to UUIDs.

DROP INDEX IF EXISTS uq_customer_client_name_phone;

DROP INDEX IF EXISTS idx_customers_client_name;
DROP INDEX IF EXISTS idx_customers_merchant_id;

ALTER TABLE customers
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS name;

ALTER TABLE customers
    ALTER COLUMN merchant_id SET NOT NULL;

ALTER TABLE customers
    RENAME COLUMN client_name TO client_id;

ALTER TABLE customers
    ALTER COLUMN client_id TYPE UUID USING client_id::UUID;

CREATE INDEX idx_customers_client_id ON customers (client_id);

CREATE INDEX idx_customers_merchant_id ON customers (merchant_id);

ALTER TABLE customers
    ADD CONSTRAINT uq_customer_client_merchant_phone
    UNIQUE (client_id, merchant_id, phone_number);
