-- user_role mirrors the USER_ROLE_USER / USER_ROLE_ADMIN model. Following the
-- project's PostgreSQL enum convention (client_status, integration_status,
-- webhook_subscription_status), the type carries the full role names so both
-- roles are explicitly representable and no additional roles can exist.
CREATE TYPE user_role AS ENUM (
    'USER_ROLE_USER',
    'USER_ROLE_ADMIN'
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name TEXT NOT NULL,

    email TEXT NOT NULL,

    password_hash TEXT NOT NULL,

    user_role user_role NOT NULL DEFAULT 'USER_ROLE_USER',

    refresh_token_hash TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    token_hash TEXT NOT NULL,

    refresh_token_hash TEXT NOT NULL,

    expires_at TIMESTAMPTZ NOT NULL,

    refresh_expires_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_access_tokens_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
        ON UPDATE RESTRICT,

    CONSTRAINT uq_access_tokens_token_hash UNIQUE (token_hash)
);

CREATE INDEX idx_access_tokens_user_id ON access_tokens (user_id);

CREATE INDEX idx_access_tokens_refresh_token_hash ON access_tokens (refresh_token_hash);