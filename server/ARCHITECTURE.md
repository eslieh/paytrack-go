# PayTrack architecture

PayTrack is a multi-tenant M-Pesa collections router. Tenants register **payment destinations** (a bank account through the bank's PayBill, any PayBill, or a Till). PayTrack sends STK pushes to customers **using the platform's own Daraja app** and routes each payment to the tenant's destination through `PartyB` and `AccountReference`. Money goes straight to the destination and never into a PayTrack account. PayTrack then tells the tenant's backend the outcome through a signed webhook.

v1 scope: **C2B collections via STK push** (Lipa Na M-Pesa Online). Backend only. A separate frontend (dashboard) uses the same REST API; see [`api/openapi.yaml`](api/openapi.yaml).

## Shape: hexagonal modular monolith

This is one Go module and one container image. It has four modules, each a hexagon (domain → app/ports → adapters), and each owns its own Postgres schema:

| Module | Owns | Inbound adapters | Outbound adapters |
|---|---|---|---|
| `identity` | users, sessions, Google identities, verification/reset/sign-in-code tokens, trusted devices, security audit log | HTTP | Postgres, bcrypt/JWT, Google OAuth, mail, rate limiter, tenant (in-process) |
| `tenant` | tenants, API keys, channels (routing destinations), bank catalog, webhook config, notification settings, setup checklist | HTTP | Postgres, identity / payment / mail (in-process) |
| `mail` | transactional email queue, templates (dashboard design language), delivery log | called in-process by other modules (`Enqueue`) | Postgres, Resend (or a log sender in dev) |
| `payment` | Payment aggregate + state machine, reconciliation | HTTP, Kafka (`mpesa.stk-callbacks.v1`) | Postgres + outbox, Daraja, tenant (in-process) |
| `callback` | raw Daraja callback inbox | HTTP (public) | Postgres + outbox |
| `notify` | webhook deliveries, retries; merchant receipt, first-payment and webhook-failure emails | Kafka (`payment.events.v1`), HTTP | Postgres, HTTP sender, tenant + mail (in-process) |

Rules:
- `domain` packages import nothing from adapters, Postgres, Kafka or HTTP.
- Modules never import another module's `domain`, and never touch another module's tables. Cross-module calls go through a **port** whose adapter is `tenantlink` (in-process today; becomes a gRPC/HTTP client if a module is extracted). Asynchronous flow uses Kafka messages defined in `internal/contracts`.
- Wiring happens only in `cmd/paytrack/wire.go` (the composition root).

```
server/
  cmd/paytrack/            main + wiring; role chosen by argv
  internal/
    contracts/             Kafka message schemas (shared language between modules)
    platform/              config, db+migrations, outbox, kafka, httpx, auth, crypto, ids, errs
    tenant/   {domain, app, adapters/{http,postgres,security}}
    payment/  {domain, app, adapters/{http,postgres,daraja,kafka,tenantlink}}
    callback/ {domain, app, adapters/{http,postgres}}
    notify/   {domain, app, adapters/{http,postgres,kafka,webhook,tenantlink}}
  migrations/              embedded SQL
  api/openapi.yaml         contract for the frontend and tenant integrations
```

## Runtime roles

One binary, deployed as separate containers per role so each scales and fails independently:

| Role | Exposes | Depends on | Notes |
|---|---|---|---|
| `api` | `:9710` REST (dashboard + tenant backends) | Postgres, Daraja | Doesn't need Kafka, so payments can still be initiated during a Kafka outage |
| `callback` | `:9711` `POST /callbacks/mpesa/stk/{token}` (public) | Postgres only | Smallest possible dependency set; must always be up |
| `worker` | `:9712` health | Postgres, Kafka, Daraja, tenant webhooks, Resend | Outbox relay, consumers, reconciler, webhook dispatcher, email dispatcher, setup reminders |
| `migrate` | none | Postgres | Runs once per deploy (advisory-locked) |

## Payment flow

```
Tenant backend / dashboard
   │ POST /v1/payments  (Idempotency-Key)
   ▼
[api] payment.Initiate ── resolve channel → destination (tenant port) ── insert payment (created)
   │                        routing snapshot + callback_token = random 256-bit
   ├──► Daraja STK push, signed with the platform app
   │      BusinessShortCode/Password = platform shortcode + passkey
   │      PartyB/AccountReference    = tenant destination
   │      CallBackURL                = …/callbacks/mpesa/stk/{callback_token}
   │       ok        → pending (CheckoutRequestID stored)
   │       rejected  → failed (returned synchronously)
   │       timeout   → stays created (settled by callback or reconciler)
   ▼
Customer approves/cancels on phone
   ▼
Safaricom ─► [callback] store raw in callback.inbox + outbox row (one tx) ─► 200 to Safaricom
                          │
                [worker] outbox relay ─► Kafka mpesa.stk-callbacks.v1 (key = CheckoutRequestID)
                          │
                [worker] payment consumer: find payment by callback_token, apply result
                          state change + outbox row payment.events.v1 (one tx)
                          │
                [worker] outbox relay ─► Kafka payment.events.v1 (key = payment id)
                          │
                [worker] notify consumer: create delivery (unique per event id)
                          │
                [worker] dispatcher ─► POST tenant webhook, HMAC-signed, retried with backoff
```

### Payment state machine

```
created ──push ok──────► pending ──result 0──────► succeeded
   │                        ├──result 1032────────► cancelled
   │                        ├──result other───────► failed
   │                        └──6 status queries,──► expired ──late success──► succeeded
   │                           no answer
   ├──push rejected───────► failed (sync, no webhook)
   └──no callback in 3 min► expired
```

`succeeded`, `failed` and `cancelled` are final. `expired` can still move to `succeeded`, so money that actually arrived is never reported as lost. Every transition except initiation failure emits a webhook event: `payment.succeeded`, `payment.failed`, `payment.cancelled`, `payment.expired`.

### Routing (platform-initiated)

Every request uses the platform Daraja app from `MPESA_*`. The channel decides only where the money goes:

| Channel type | TransactionType | PartyB | AccountReference |
|---|---|---|---|
| `bank` | CustomerPayBillOnline | bank's PayBill (from `GET /v1/banks`) | the channel's bank account number. It is fixed, and a payment can't override it, so money can't be redirected to another account |
| `paybill` | CustomerPayBillOnline | PayBill number | the payment's `account_reference`, else the channel default |
| `till` | CustomerBuyGoodsOnline | Till number | the payment's value, else "Till Payment" |

Each payment stores a snapshot of `destination_type`, `PartyB` and `bank_name`, so later channel edits don't rewrite history.

### Daraja details that shape the design

- **Callback routing by our own secret.** Daraja does not sign callbacks. Each payment gets a random 256-bit `callback_token` embedded in the CallBackURL. Callbacks are matched by token, and the CheckoutRequestID is then checked against it. Forged callbacks would have to guess the token. In production, also set `CALLBACK_ALLOWED_CIDRS` to Safaricom's published IPs.
- **Callbacks get lost or duplicated.** The reconciler (in the worker) runs STK status queries on overdue pending payments with backoff: 45s, 1m, 2m, 5m, 10m and 20m. The state machine ignores duplicates.
- **Ambiguous push failures.** A timeout or 5xx from Daraja does not prove the prompt wasn't sent. The payment stays `created`. If a callback arrives it is still matched by token; otherwise the payment expires.
- **Token cache.** The platform app's OAuth token is cached in each process and refreshed a minute early. `singleflight` stops a burst of requests from fetching many tokens, and a 401 invalidates the cache and retries once.
- **Not sent vs. ambiguous.** If the OAuth token can't be obtained, the push was never sent, so the payment fails immediately with "M-Pesa is temporarily unavailable" and the tenant can retry.
- **Secrets.** The platform Daraja credentials live only in env (use a secrets manager in production). Webhook signing secrets are AES-256-GCM encrypted at rest with `ENCRYPTION_KEY`.

## Reliability patterns

| Concern | Mechanism |
|---|---|
| Client retries `POST /v1/payments` | `Idempotency-Key` required; unique on (tenant, key); the same body returns the original (`Idempotent-Replayed: true`), a different body returns 409 |
| State change and event publish atomicity | Transactional **outbox** table; the relay publishes with `FOR UPDATE SKIP LOCKED` |
| Never lose a callback | Raw body stored in `callback.inbox` before we reply 200 |
| Concurrent writers (callback vs reconciler vs initiator) | Optimistic locking (`version`), reload and retry |
| Consumer failures | At-least-once: commit after the handler succeeds; 5 retries, then `<topic>.dlq`; handlers are idempotent |
| Tenant endpoint down | Retries over ~16h (10s → 8h), then `failed`; manual redeliver via API |
| Webhook authenticity | `PayTrack-Signature: t=<unix>,v1=hex(HMAC-SHA256(secret, "<t>.<body>"))`; secret rotatable |
| Webhook dedupe | `PayTrack-Event-Id` header and `id` in the body are stable across retries |
| SSRF via webhook URLs | The dialer refuses private, loopback and link-local IPs after DNS resolution; no redirects |

## Auth model

Two kinds of credential both resolve to a `tenant_id`, which scopes every query. The tenant ID never comes from request input.

- **Tenant backends** use API keys (`ptk_…`). Keys are stored as SHA-256 hashes, can be revoked, and can only be created by a verified dashboard user.
- **Dashboard users** have revocable sessions (`identity.sessions`):
  - **Access token:** a 15-minute HS256 JWT carrying the session ID. The session row is checked on every request, so signing out takes effect immediately.
  - **Refresh token:** an httpOnly `SameSite=Lax` cookie (`pt_refresh`, 30 days) that rotates on every `POST /v1/auth/refresh`. Reusing an already-rotated token revokes the session. A 20-second grace window covers two tabs refreshing at once.
  - **Logout** revokes the session on the server and clears the cookie.
  - **Where it lives:** the dashboard calls the API through its own `/api` path (a Next.js rewrite). That keeps the cookie first-party on every dashboard domain, and the access token stays in memory only.

### Sign-in

| Method | Flow |
|---|---|
| Email + password | Correct password on an **untrusted device** with two-step on (the default) → `{mfa_required, challenge}`, and a 6-digit code is emailed. The code is HMAC-hashed, expires in 10 minutes, allows 5 attempts, and can be resent after 30 seconds (up to 5 sends). `remember_device` sets a 30-day `pt_device` cookie. A trusted device, or two-step turned off, signs in directly and sends a "new sign-in" alert for untrusted devices. |
| Google | Authorization-code flow with PKCE and nonce, run by the backend with the client secret. The redirect URI is `<dashboard origin>/api/v1/auth/google/callback`, and an OAuth state cookie binds the flow to the browser. A linked identity signs in. A matching **verified** email links the Google account automatically. A matching **unverified** email is taken over safely: the unproven password is dropped and every session revoked. A new email goes to `/auth/google/complete` to name the business. Google sign-in skips email codes, since Google applies its own 2-step. |

### Verification and recovery

- **Sign-up** creates an unverified owner and emails a verification link (24 hours, single use, stored as a hash). Until the owner verifies, `RequireVerified` blocks creating API keys, channels and payments. Verifying sends the welcome email.
- **Forgot password** always returns 202. The reset link lasts 30 minutes and works once. Completing a reset sets the new password, marks the email verified, revokes every session and trusted device, sends a "password changed" email and signs the user in.
- **Changing the password** (or setting a first one on a Google-only account) signs out the other sessions. Turning two-step off requires the password and sends an alert email.

### Abuse controls and audit

- **Rate limits** use a Postgres fixed-window limiter, shared by every replica:
  - login: 30 per IP and 10 per email per 15 minutes
  - code checks: 30 per IP per 15 minutes
  - sign-up: 10 per IP per hour
  - forgot password: 10 per IP and 3 per email per hour
  - verification resends: 5 per user per hour
  - responses are 429 `rate_limited`
- **Audit log:** `identity.security_events` records sign-ins, failures, codes, password and two-step changes, Google link/unlink, session revocations and refresh-token reuse. It's shown under Settings → Security.

## Transactional email

- **Queueing:** modules call their `Mailer` port, which the mail module's `Enqueue` satisfies. The template is rendered once up front, so a bad template or data bug fails at the call site, then the message is queued in `mail.messages`.
- **Sending:** the worker's email dispatcher sends through Resend (`Idempotency-Key` = message ID), using `SKIP LOCKED` leases and backoff (30s → 2h). 4xx responses other than 429 are permanent failures.
- **Dedupe:** a `dedupe_key` makes events safe to process more than once (`receipt:<payment>`, `first_payment:<tenant>`, `setup_reminder:<tenant>:<day>`, `welcome:<user>`).
- **Templates:** Go `html/template` files embedded in the binary (`internal/mail/templates`), sharing one layout built from the dashboard's design tokens (`tokens.go`, copied from `globals.css`):
  - Figtree, the indigo `#6472f3` wordmark and button, a white 16px card with a `#e9ebf4` border, and the soft indigo gradient
  - the receipt email mirrors `/receipt`
  - every email also has a plain-text part, and data is auto-escaped
- **Without `RESEND_API_KEY`**, emails are written to the log instead of sent (development).

| Group | Templates | Trigger |
|---|---|---|
| Security (always sent) | `verify_email`, `login_code`, `password_reset`, `password_changed`, `new_signin`, `two_step_disabled`, `google_linked` | identity use cases |
| Lifecycle | `welcome`, `setup_reminder` (day 1 and day 3 if the checklist is incomplete, verified owners only), `first_payment` | verification, an hourly worker job, the first `payment.succeeded` |
| Payments | `payment_receipt`, sent to the merchant when **payment receipts** are on, at `receipt_email` or the owner's address | `payment.succeeded` in notify |
| Operations | `webhook_failing`, when a delivery exhausts its retries (on by default) | notify dispatcher |

Settings → Notifications shows a live preview of the real templates (`GET /v1/settings/notifications/preview`) and can send a test receipt.

## High availability on EC2

Target layout, with every role running as a container on at least two instances across at least two AZs:

```
            Route53
               │
      ┌────────┴────────┐
   ALB (api)         ALB (callback, public, Safaricom IP allowlist at SG/WAF)
      │                  │
  ┌───┴───┐          ┌───┴───┐
  api x2+  │          callback x2+       worker x2+ (no inbound traffic)
  (AZ a/b) │          (AZ a/b)           (AZ a/b)
           └─────────┬──────────────────────┘
          RDS Postgres Multi-AZ      Amazon MSK (3 brokers, RF=3, min.insync=2)
```

- **Stateless processes.** Any replica can die. Background work is claimed with `SKIP LOCKED` plus leases, so there's no leader election and an expired lease just gets picked up by another replica.
- **Graceful shutdown.** SIGTERM drains HTTP (25s) and stops consumers before committing in-flight work. Uncommitted messages are redelivered.
- **Health checks.** `/healthz` (liveness) and `/readyz` (DB ping) on every role; the worker's are on `:9712`.
- **Failure isolation.**
  - Kafka down: the API and callbacks still work, and the outbox buffers events.
  - Workers down: callbacks are still stored and get processed on recovery.
  - Tenant endpoint down: only that tenant's deliveries back off.
- **Postgres** is the single source of truth. Use RDS Multi-AZ with PITR backups, and point the API's read-heavy list endpoints at a read replica later if needed.
- **Kafka** in production: `KAFKA_REPLICATION_FACTOR=3`. Partition counts (`KAFKA_PARTITIONS`) cap consumer parallelism per group.
- **Deploying on EC2.** Start with `docker compose` per instance behind ALBs; ECS on EC2 is the natural next step without changing the image.

## Next steps (not built yet)

- Per-tenant rate limiting (ALB/WAF or Redis token bucket) and per-channel circuit breaker for Daraja.
- Shared token cache (Redis) once API/worker replica count makes per-process OAuth calls noticeable.
- Bank catalog in the database (admin-managed) instead of code; verify each bank's PayBill before production.
- OpenTelemetry tracing and metrics: stuck-pending count, callback rate, DLQ depth, webhook failure rate.
- Team members and roles per tenant; TOTP authenticator apps as a stronger second factor.
- Resend bounce/complaint webhooks → a suppression list, plus email delivery status in the dashboard.
- Daily payment summary email.
- C2B register-URL flow (customers paying the PayBill directly without an STK prompt) as a new inbound adapter.
- Integration tests against Postgres and Kafka (testcontainers).
