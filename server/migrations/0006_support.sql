-- Support: tickets between tenant users and platform staff, their message
-- threads (with staff-only internal notes) and free-form product feedback.
CREATE SCHEMA IF NOT EXISTS support;

CREATE TABLE support.tickets (
    id                text PRIMARY KEY,
    number            bigserial   NOT NULL UNIQUE, -- human reference, e.g. #1042
    tenant_id         text        NOT NULL,
    user_id           text        NOT NULL,
    -- Snapshots so staff lists need no cross-module lookups.
    requester_email   text        NOT NULL,
    tenant_name       text        NOT NULL,
    subject           text        NOT NULL,
    category          text        NOT NULL,
    priority          text        NOT NULL DEFAULT 'normal',
    status            text        NOT NULL CHECK (status IN ('awaiting_staff', 'awaiting_user', 'resolved', 'closed')),
    first_response_at timestamptz,
    last_message_at   timestamptz NOT NULL,
    resolved_at       timestamptz,
    rating            int CHECK (rating BETWEEN 1 AND 5),
    rating_comment    text        NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL
);
CREATE INDEX tickets_tenant ON support.tickets (tenant_id, id DESC);
CREATE INDEX tickets_status ON support.tickets (status, id DESC);

CREATE TABLE support.messages (
    id          text PRIMARY KEY,
    ticket_id   text        NOT NULL REFERENCES support.tickets (id),
    author_kind text        NOT NULL CHECK (author_kind IN ('user', 'staff')),
    author_id   text        NOT NULL,
    author_name text        NOT NULL,
    body        text        NOT NULL,
    internal    boolean     NOT NULL DEFAULT false, -- staff-only note
    created_at  timestamptz NOT NULL
);
CREATE INDEX messages_ticket ON support.messages (ticket_id, id);

CREATE TABLE support.feedback (
    id         text PRIMARY KEY,
    tenant_id  text        NOT NULL,
    user_id    text        NOT NULL,
    user_email text        NOT NULL,
    rating     int CHECK (rating BETWEEN 1 AND 5),
    message    text        NOT NULL,
    page       text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);
CREATE INDEX feedback_created ON support.feedback (id DESC);
