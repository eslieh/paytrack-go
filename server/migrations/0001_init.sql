-- Each module owns one schema. Modules never read or write another
-- module's tables; they go through ports (in-process) or Kafka events.
CREATE SCHEMA IF NOT EXISTS tenant;
CREATE SCHEMA IF NOT EXISTS payment;
CREATE SCHEMA IF NOT EXISTS callback;
CREATE SCHEMA IF NOT EXISTS notify;

-- Shared transactional outbox (see internal/platform/outbox).
CREATE TABLE outbox (
    id           bigserial PRIMARY KEY,
    topic        text        NOT NULL,
    key          text        NOT NULL,
    payload      jsonb       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);
CREATE INDEX outbox_unpublished ON outbox (id) WHERE published_at IS NULL;

-- ---------------------------------------------------------------- tenant
CREATE TABLE tenant.tenants (
    id                 text PRIMARY KEY,
    name               text        NOT NULL,
    webhook_url        text,
    webhook_secret_enc bytea,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenant.users (
    id            text PRIMARY KEY,
    tenant_id     text        NOT NULL REFERENCES tenant.tenants (id),
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    role          text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email)
);

CREATE TABLE tenant.api_keys (
    id           text PRIMARY KEY,
    tenant_id    text        NOT NULL REFERENCES tenant.tenants (id),
    name         text        NOT NULL,
    prefix       text        NOT NULL,
    hash         bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);
CREATE INDEX api_keys_tenant ON tenant.api_keys (tenant_id);

CREATE TABLE tenant.channels (
    id              text PRIMARY KEY,
    tenant_id       text        NOT NULL REFERENCES tenant.tenants (id),
    name            text        NOT NULL,
    type            text        NOT NULL CHECK (type IN ('paybill', 'till')),
    environment     text        NOT NULL CHECK (environment IN ('sandbox', 'production')),
    short_code      text        NOT NULL,
    till_number     text        NOT NULL DEFAULT '',
    credentials_enc bytea       NOT NULL,
    active          boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX channels_tenant ON tenant.channels (tenant_id);

-- ---------------------------------------------------------------- payment
CREATE TABLE payment.payments (
    id                  text PRIMARY KEY,
    tenant_id           text        NOT NULL,
    channel_id          text        NOT NULL,
    idempotency_key     text        NOT NULL,
    request_hash        text        NOT NULL,
    status              text        NOT NULL,
    amount              bigint      NOT NULL CHECK (amount > 0),
    phone               text        NOT NULL,
    account_reference   text        NOT NULL,
    description         text        NOT NULL,
    metadata            jsonb       NOT NULL DEFAULT '{}',
    callback_token      text        NOT NULL,
    merchant_request_id text        NOT NULL DEFAULT '',
    checkout_request_id text        NOT NULL DEFAULT '',
    mpesa_receipt       text        NOT NULL DEFAULT '',
    result_code         int,
    result_desc         text        NOT NULL DEFAULT '',
    failure_reason      text        NOT NULL DEFAULT '',
    check_attempts      int         NOT NULL DEFAULT 0,
    next_check_at       timestamptz,
    version             int         NOT NULL DEFAULT 1,
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    completed_at        timestamptz,
    CONSTRAINT payments_idempotency_key UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT payments_callback_token_key UNIQUE (callback_token)
);
CREATE INDEX payments_tenant_id ON payment.payments (tenant_id, id DESC);
CREATE INDEX payments_checkout ON payment.payments (checkout_request_id) WHERE checkout_request_id <> '';
CREATE INDEX payments_due_checks ON payment.payments (next_check_at)
    WHERE status IN ('created', 'pending') AND next_check_at IS NOT NULL;

-- ---------------------------------------------------------------- callback
-- Raw Daraja callbacks are stored before we acknowledge them, so nothing is
-- lost even if every downstream component is down.
CREATE TABLE callback.inbox (
    id                  bigserial PRIMARY KEY,
    callback_token      text        NOT NULL,
    checkout_request_id text        NOT NULL DEFAULT '',
    raw                 bytea       NOT NULL,
    parse_error         text        NOT NULL DEFAULT '',
    remote_addr         text        NOT NULL DEFAULT '',
    received_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inbox_checkout ON callback.inbox (checkout_request_id);

-- ---------------------------------------------------------------- notify
CREATE TABLE notify.deliveries (
    id               text PRIMARY KEY,
    event_id         text        NOT NULL,
    tenant_id        text        NOT NULL,
    payment_id       text        NOT NULL,
    event_type       text        NOT NULL,
    payload          bytea       NOT NULL,
    status           text        NOT NULL,
    attempts         int         NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz,
    last_error       text        NOT NULL DEFAULT '',
    last_status_code int         NOT NULL DEFAULT 0,
    delivered_at     timestamptz,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    CONSTRAINT deliveries_event_key UNIQUE (event_id)
);
CREATE INDEX deliveries_tenant ON notify.deliveries (tenant_id, id DESC);
CREATE INDEX deliveries_due ON notify.deliveries (next_attempt_at)
    WHERE status IN ('pending', 'delivering');
