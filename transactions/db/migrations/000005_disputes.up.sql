CREATE TYPE dispute_status AS ENUM (
    'NEEDS_RESPONSE',
    'UNDER_REVIEW',
    'RESOLVED'
);

CREATE TABLE disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    deposit_id UUID NOT NULL,

    client_name TEXT NOT NULL,

    dispute_type TEXT NOT NULL,

    amount NUMERIC(18,2) NOT NULL,

    currency VARCHAR(3) NOT NULL
        CHECK (currency ~ '^[A-Z]{3}$'),

    status dispute_status NOT NULL DEFAULT 'NEEDS_RESPONSE',

    evidence_submitted BOOLEAN NOT NULL DEFAULT false,

    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    due_at TIMESTAMPTZ NOT NULL,

    resolved_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_dispute_deposit
        FOREIGN KEY (deposit_id)
        REFERENCES deposits(id)
        ON DELETE RESTRICT
        ON UPDATE RESTRICT
);

CREATE INDEX idx_disputes_deposit_id ON disputes (deposit_id);
CREATE INDEX idx_disputes_status ON disputes (status);
CREATE INDEX idx_disputes_client_name ON disputes (client_name);
CREATE INDEX idx_disputes_opened_at ON disputes (opened_at);
