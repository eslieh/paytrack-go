# PayTrack server

A multi-tenant M-Pesa STK push router. Tenants register their Daraja channels, PayTrack sends STK pushes for them, and their backends receive signed webhooks with the outcome.

- Design and HA notes: [ARCHITECTURE.md](ARCHITECTURE.md)
- API contract (for the frontend and tenant integrations): [api/openapi.yaml](api/openapi.yaml)
- The original single-tenant demo is kept in [legacy/mpesastk](legacy/mpesastk)

## Run locally

1. Copy `.env.example` to `.env` and fill in the following.
   - `DATABASE_URL`: any Postgres 15 or later. SQLAlchemy-style `postgresql+asyncpg://` URLs are accepted.
   - `ENCRYPTION_KEY`: generate with `openssl rand -base64 32`.
   - `JWT_SECRET`: generate with `openssl rand -base64 48`.
   - `PUBLIC_CALLBACK_BASE_URL`: a public HTTPS URL that forwards to the callback role (`:8081`), for example `ngrok http 8081`. Daraja won't call `localhost`.
2. Start everything:
   ```sh
   docker compose up --build        # kafka + migrate + api + callback + worker
   ```
   Alternatively, run Kafka in Docker and the app on the host:
   ```sh
   docker compose up -d kafka
   set -a; source .env; set +a
   go run ./cmd/paytrack all        # migrations + every role in one process
   ```
3. Use the API:
   ```sh
   # dashboard sign-up → JWT
   curl -XPOST localhost:8080/v1/auth/register -d '{"business_name":"Shop","email":"me@shop.co.ke","password":"a-long-password"}'
   # with the JWT: create an API key for the tenant backend, set the webhook URL
   curl -XPOST localhost:8080/v1/api-keys -H "Authorization: Bearer $JWT" -d '{"name":"backend"}'
   curl -XPUT  localhost:8080/v1/webhook  -H "Authorization: Bearer $JWT" -d '{"url":"https://shop.co.ke/hooks/paytrack"}'
   # with the API key: register a channel and charge a customer
   curl -XPOST localhost:8080/v1/channels -H "Authorization: Bearer $KEY" -d '{"name":"Sandbox","type":"paybill","environment":"sandbox","short_code":"174379","consumer_key":"…","consumer_secret":"…","passkey":"…"}'
   curl -XPOST localhost:8080/v1/payments -H "Authorization: Bearer $KEY" -H "Idempotency-Key: order-123" \
        -d '{"channel_id":"chn_…","amount":10,"phone":"0712345678","account_reference":"ORDER123"}'
   ```

## Roles

`paytrack <migrate|api|callback|worker|all>`: one image, with the role chosen per container. See ARCHITECTURE.md for what each role does.

| Role | Port | Health |
|---|---|---|
| api | `API_ADDR` (8080) | `/healthz`, `/readyz` |
| callback | `CALLBACK_ADDR` (8081) | `/healthz`, `/readyz` |
| worker | `WORKER_ADDR` (8082) | `/healthz`, `/readyz` |

## Verifying webhooks (tenant side)

```
PayTrack-Signature: t=1760512345,v1=5f2b…
expected = hex(HMAC_SHA256(signing_secret, t + "." + raw_body))
```

Reject the request if the signature doesn't match or `t` is more than 5 minutes old. Dedupe on `PayTrack-Event-Id`, and respond 2xx quickly.

## Tests

```sh
go test ./...
```
