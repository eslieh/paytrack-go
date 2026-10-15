-- Records a value encrypted with the ENCRYPTION_KEY this database was
-- initialised with, so processes started with a different key refuse to
-- boot instead of failing on every decrypt.
CREATE TABLE encryption_key_check (
    id         int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    canary     bytea       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
