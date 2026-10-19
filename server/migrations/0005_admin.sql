-- Admin portal foundations: tenant suspension, disabled users, platform
-- staff (separate from tenant users), staff sessions and an audit log of
-- every staff action.

-- A suspended tenant's API keys stop working and its dashboard becomes
-- read-only. Payments already in flight still complete.
ALTER TABLE tenant.tenants
    ADD COLUMN status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    ADD COLUMN suspended_at     timestamptz,
    ADD COLUMN suspended_reason text NOT NULL DEFAULT '';

-- A disabled user cannot sign in; their sessions are revoked.
ALTER TABLE identity.users
    ADD COLUMN disabled_at     timestamptz,
    ADD COLUMN disabled_reason text NOT NULL DEFAULT '';

CREATE SCHEMA IF NOT EXISTS admin;

CREATE TABLE admin.staff (
    id            text PRIMARY KEY,
    email         text        NOT NULL,
    name          text        NOT NULL DEFAULT '',
    password_hash text        NOT NULL,
    last_login_at timestamptz,
    disabled_at   timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT staff_email_key UNIQUE (email)
);

-- Sign-in always needs a password and an emailed code.
CREATE TABLE admin.login_challenges (
    id          text PRIMARY KEY,
    staff_id    text        NOT NULL,
    code_hash   bytea       NOT NULL,
    attempts    int         NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz
);

-- Opaque cookie sessions with an idle timeout and an absolute lifetime.
CREATE TABLE admin.sessions (
    id           text PRIMARY KEY,
    staff_id     text        NOT NULL,
    token_hash   bytea       NOT NULL UNIQUE,
    ip           text        NOT NULL DEFAULT '',
    user_agent   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz
);
CREATE INDEX sessions_staff ON admin.sessions (staff_id);

CREATE TABLE admin.audit_log (
    id          bigserial PRIMARY KEY,
    staff_id    text        NOT NULL,
    action      text        NOT NULL,
    target_type text        NOT NULL DEFAULT '',
    target_id   text        NOT NULL DEFAULT '',
    reason      text        NOT NULL DEFAULT '',
    data        jsonb       NOT NULL DEFAULT '{}',
    ip          text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL
);
CREATE INDEX audit_log_target ON admin.audit_log (target_type, target_id, id DESC);
