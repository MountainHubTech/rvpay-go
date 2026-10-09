-- 000007_payment_events.up.sql
-- Durable outbound event outbox for the RVPay → HighLevel Inbound Webhook
-- workflow delivery ("rvpay.payment.completed").
--
-- The event is inserted in the SAME database transaction as the authoritative
-- PawaPay COMPLETED deposit transition, so a confirmed successful payment
-- always produces exactly one durable event. A background worker
-- (transactions/ghldeliver) delivers it asynchronously; HighLevel
-- availability never affects the payment state.
--
-- Schema decisions:
--   deposit_id UNIQUE   one logical event per payment; repeated PawaPay
--                       callbacks cannot create a second event (the insert
--                       uses ON CONFLICT (deposit_id) DO NOTHING and the
--                       finalize transition is terminal anyway).
--   event_id            stable UUID; every delivery retry reuses the SAME
--                       event id, idempotency key and payload.
--   payload JSONB       the exact agreed outbound JSON contract, snapshotted
--                       at emission; retries resend the identical bytes.
--   delivery_status     'pending' | 'delivered' | 'failed'
--   attempts/next_retry_at/last_error/delivered_at   bounded retry bookkeeping.

CREATE TYPE payment_event_delivery_status AS ENUM ('pending', 'delivered', 'failed');

CREATE TABLE payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    deposit_id UUID NOT NULL UNIQUE,

    event_id UUID NOT NULL UNIQUE,

    event_type TEXT NOT NULL,

    idempotency_key TEXT NOT NULL,

    payload JSONB NOT NULL,

    delivery_status payment_event_delivery_status NOT NULL DEFAULT 'pending',

    attempts INT NOT NULL DEFAULT 0,

    last_error TEXT,

    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    delivered_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_events_delivery ON payment_events (delivery_status, next_retry_at);
