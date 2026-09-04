-- 000004_customer_client_name.up.sql
-- Moves the customer tenant association from the internal clients UUID
-- (client_id) to the external client name (client_name), matching the
-- deposits identifier model introduced in migration 000003. Adds the
-- dashboard-required customer attributes (name, address) and relaxes the
-- merchant reference so customers can be created from the external payment
-- flow, where no RVPay merchant UUID exists yet.
--
-- Data preservation: existing client_id UUID values are preserved as their
-- canonical text form. The merchant foreign key is kept for non-NULL values;
-- existing rows are untouched.
--
-- FK decision: NO foreign key is created between customers.client_name and
-- clients.client_name. clients.client_name is not guaranteed unique (no
-- UNIQUE constraint exists), so a database-level FK cannot safely be
-- introduced. Client scoping is enforced at the repository/service level.

ALTER TABLE customers
    DROP CONSTRAINT IF EXISTS uq_customer_client_merchant_phone;

DROP INDEX IF EXISTS idx_customers_client_id;

ALTER TABLE customers
    RENAME COLUMN client_id TO client_name;

ALTER TABLE customers
    ALTER COLUMN client_name TYPE TEXT USING client_name::TEXT,
    ALTER COLUMN merchant_id DROP NOT NULL,
    ADD COLUMN name TEXT,
    ADD COLUMN address TEXT;

CREATE INDEX idx_customers_client_name ON customers (client_name);

CREATE INDEX idx_customers_merchant_id ON customers (merchant_id);

CREATE UNIQUE INDEX uq_customer_client_name_phone
    ON customers (client_name, phone_number);
