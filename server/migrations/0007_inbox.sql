-- In-app notifications shown under the dashboard's bell. The ID is the
-- contracts.Notification ID, so redelivered messages are ignored.
CREATE SCHEMA IF NOT EXISTS inbox;

CREATE TABLE inbox.notifications (
    id         text PRIMARY KEY,
    tenant_id  text        NOT NULL,
    user_id    text        NOT NULL DEFAULT '', -- empty: everyone in the tenant
    kind       text        NOT NULL,
    title      text        NOT NULL,
    body       text        NOT NULL DEFAULT '',
    link       text        NOT NULL DEFAULT '',
    read_at    timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX notifications_tenant ON inbox.notifications (tenant_id, created_at DESC);
CREATE INDEX notifications_unread ON inbox.notifications (tenant_id) WHERE read_at IS NULL;
