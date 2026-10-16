-- Identity: users move out of the tenant module, plus revocable sessions,
-- Google identities, single-use tokens (verification, reset, login codes,
-- OAuth state), trusted devices and a security audit log.
CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.users (
    id                text PRIMARY KEY,
    tenant_id         text        NOT NULL,
    email             text        NOT NULL,
    password_hash     text        NOT NULL DEFAULT '', -- empty for Google-only accounts
    role              text        NOT NULL,
    email_verified_at timestamptz,
    two_step_enabled  boolean     NOT NULL DEFAULT true,
    last_login_at     timestamptz,
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT users_email_key UNIQUE (email)
);
CREATE INDEX users_tenant ON identity.users (tenant_id);

-- Existing accounts predate verification; treat them as verified.
INSERT INTO identity.users (id, tenant_id, email, password_hash, role, email_verified_at, created_at, updated_at)
SELECT id, tenant_id, email, password_hash, role, created_at, created_at, created_at FROM tenant.users;
DROP TABLE tenant.users;

CREATE TABLE identity.sessions (
    id                 text PRIMARY KEY,
    user_id            text        NOT NULL,
    tenant_id          text        NOT NULL,
    refresh_hash       bytea       NOT NULL UNIQUE,
    prev_refresh_hash  bytea, -- detects reuse of a rotated refresh token
    auth_method        text        NOT NULL,
    user_agent         text        NOT NULL DEFAULT '',
    ip                 text        NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL,
    last_seen_at       timestamptz NOT NULL,
    expires_at         timestamptz NOT NULL,
    revoked_at         timestamptz
);
CREATE INDEX sessions_user ON identity.sessions (user_id);
CREATE INDEX sessions_prev_refresh ON identity.sessions (prev_refresh_hash) WHERE prev_refresh_hash IS NOT NULL;

CREATE TABLE identity.oauth_identities (
    provider   text        NOT NULL,
    subject    text        NOT NULL,
    user_id    text        NOT NULL,
    email      text        NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (provider, subject)
);
CREATE INDEX oauth_identities_user ON identity.oauth_identities (user_id);

CREATE TABLE identity.tokens (
    id           text PRIMARY KEY,
    user_id      text        NOT NULL DEFAULT '',
    purpose      text        NOT NULL,
    token_hash   bytea       NOT NULL UNIQUE,
    code_hash    bytea,
    attempts     int         NOT NULL DEFAULT 0,
    sends        int         NOT NULL DEFAULT 1,
    last_sent_at timestamptz NOT NULL,
    data         jsonb       NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    consumed_at  timestamptz
);
CREATE INDEX tokens_user_purpose ON identity.tokens (user_id, purpose);

CREATE TABLE identity.trusted_devices (
    id           text PRIMARY KEY,
    user_id      text        NOT NULL,
    token_hash   bytea       NOT NULL UNIQUE,
    user_agent   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL
);
CREATE INDEX trusted_devices_user ON identity.trusted_devices (user_id);

CREATE TABLE identity.security_events (
    id         bigserial PRIMARY KEY,
    user_id    text        NOT NULL,
    tenant_id  text        NOT NULL,
    type       text        NOT NULL,
    ip         text        NOT NULL DEFAULT '',
    user_agent text        NOT NULL DEFAULT '',
    data       jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX security_events_user ON identity.security_events (user_id, id DESC);

-- Fixed-window rate limiter shared by all replicas.
CREATE TABLE rate_limits (
    key          text        NOT NULL,
    window_start timestamptz NOT NULL,
    count        int         NOT NULL,
    PRIMARY KEY (key, window_start)
);

-- Mail: queue + delivery log for transactional email.
CREATE SCHEMA IF NOT EXISTS mail;
CREATE TABLE mail.messages (
    id                  text PRIMARY KEY,
    dedupe_key          text,
    template            text        NOT NULL,
    category            text        NOT NULL,
    to_email            text        NOT NULL,
    to_name             text        NOT NULL DEFAULT '',
    data                jsonb       NOT NULL DEFAULT '{}',
    status              text        NOT NULL,
    attempts            int         NOT NULL DEFAULT 0,
    next_attempt_at     timestamptz,
    provider_message_id text        NOT NULL DEFAULT '',
    last_error          text        NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    sent_at             timestamptz,
    CONSTRAINT messages_dedupe_key UNIQUE (dedupe_key)
);
CREATE INDEX messages_due ON mail.messages (next_attempt_at) WHERE status IN ('queued', 'sending');

-- Tenant notification preferences (merchant receipts, alerts).
CREATE TABLE tenant.notification_settings (
    tenant_id              text PRIMARY KEY,
    payment_receipts       boolean     NOT NULL DEFAULT false,
    receipt_email          text        NOT NULL DEFAULT '',
    webhook_failure_alerts boolean     NOT NULL DEFAULT true,
    updated_at             timestamptz NOT NULL DEFAULT now()
);
