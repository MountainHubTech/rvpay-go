-- 000003_external_deposit_identifiers.down.sql
-- Reverses 000003. NOTE: the down migration only succeeds if every stored
-- identifier value is a valid UUID representation (i.e. no external
-- HighLevel identifiers were persisted).

DROP INDEX IF EXISTS idx_deposits_client_name;
DROP INDEX IF EXISTS idx_deposits_customer_id;
DROP INDEX IF EXISTS idx_deposits_merchant_id;

ALTER TABLE deposits
    ALTER COLUMN client_name TYPE UUID USING client_name::uuid,
    ALTER COLUMN customer_id TYPE UUID USING customer_id::uuid,
    ALTER COLUMN customer_id SET NOT NULL,
    ALTER COLUMN merchant_id TYPE UUID USING merchant_id::uuid,
    ALTER COLUMN merchant_id SET NOT NULL;

ALTER TABLE deposits
    RENAME COLUMN client_name TO client_id;

CREATE INDEX idx_deposits_client_id ON deposits (client_id);

CREATE INDEX idx_deposits_customer_id ON deposits (customer_id);

CREATE INDEX idx_deposits_merchant_id ON deposits (merchant_id);

ALTER TABLE deposits
    ADD CONSTRAINT fk_deposit_customer
        FOREIGN KEY (customer_id)
        REFERENCES customers(id)
        ON DELETE RESTRICT
        ON UPDATE RESTRICT,
    ADD CONSTRAINT fk_deposit_merchant
        FOREIGN KEY (merchant_id)
        REFERENCES merchants(id)
        ON DELETE RESTRICT
        ON UPDATE RESTRICT;