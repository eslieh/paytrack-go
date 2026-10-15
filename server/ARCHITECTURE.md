# PayTrack architecture

PayTrack is a multi-tenant M-Pesa collections router. Tenants register **payment destinations** (a bank account through the bank's PayBill, any PayBill, or a Till). PayTrack sends STK pushes to customers **using the platform's own Daraja app** and routes each payment to the tenant's destination through `PartyB` and `AccountReference`. Money goes straight to the destination and never into a PayTrack account. PayTrack then tells the tenant's backend the outcome through a signed webhook.

v1 scope: **C2B collections via STK push** (Lipa Na M-Pesa Online). Backend only. A separate frontend (dashboard) uses the same REST API; see [`api/openapi.yaml`](api/openapi.yaml).

## Shape: hexagonal modular monolith

This is one Go module and one container image. It has four modules, each a hexagon (domain → app/ports → adapters), and each owns its own Postgres schema:

| Module | Owns | Inbound adapters | Outbound adapters |
|---|---|---|---|
| `tenant` | tenants, users, API keys, channels (routing destinations), bank catalog, webhook config | HTTP | Postgres, bcrypt/JWT |
| `payment` | Payment aggregate + state machine, reconciliation | HTTP, Kafka (`mpesa.stk-callbacks.v1`) | Postgres + outbox, Daraja, tenant (in-process) |
| `callback` | raw Daraja callback inbox | HTTP (public) | Postgres + outbox |
| `notify` | webhook deliveries, retries | Kafka (`payment.events.v1`), HTTP | Postgres, HTTP sender, tenant (in-process) |

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
| `api` | `:8080` REST (dashboard + tenant backends) | Postgres, Daraja | Doesn't need Kafka, so payments can still be initiated during a Kafka outage |
| `callback` | `:8081` `POST /callbacks/mpesa/stk/{token}` (public) | Postgres only | Smallest possible dependency set; must always be up |
| `worker` | `:8082` health | Postgres, Kafka, Daraja, tenant webhooks | Outbox relay, consumers, reconciler, webhook dispatcher |
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

- **Dashboard users** register or log in with email and password and receive a JWT (HS256, `JWT_TTL`). They manage API keys, channels and webhook settings, and view payments and deliveries.
- **Tenant backends** use API keys (`ptk_…`, stored as SHA-256 hashes, revocable) to create and read payments and channels.
- Both resolve to a `tenant_id`, which every query is scoped by. The tenant ID never comes from request input.
- API key management and secret rotation require a user session.

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
- **Health checks.** `/healthz` (liveness) and `/readyz` (DB ping) on every role; the worker's are on `:8082`.
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
- Team members and roles per tenant; password reset; refresh tokens.
- C2B register-URL flow (customers paying the PayBill directly without an STK prompt) as a new inbound adapter.
- Integration tests against Postgres and Kafka (testcontainers).
