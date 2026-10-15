-- STK pushes are initiated with the platform's own Daraja app; tenants no
-- longer supply credentials. A channel is now a routing destination
-- (PartyB + AccountReference): a bank's PayBill + account number, a PayBill
-- + account, or a Till.
ALTER TABLE tenant.channels
    DROP COLUMN credentials_enc,
    DROP COLUMN environment,
    DROP COLUMN short_code,
    DROP COLUMN till_number,
    DROP CONSTRAINT channels_type_check,
    ADD COLUMN party_b        text NOT NULL,
    ADD COLUMN bank_name      text NOT NULL DEFAULT '',
    ADD COLUMN account_number text NOT NULL DEFAULT '',
    ADD CONSTRAINT channels_type_check CHECK (type IN ('bank', 'paybill', 'till'));

-- Snapshot of where each payment was routed, independent of later channel edits.
ALTER TABLE payment.payments
    ADD COLUMN destination_type text NOT NULL,
    ADD COLUMN party_b          text NOT NULL,
    ADD COLUMN bank_name        text NOT NULL DEFAULT '';
