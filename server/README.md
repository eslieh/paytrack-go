# PayTrack server

A multi-tenant M-Pesa STK push router. Tenants register payment destinations (a bank account, a PayBill, or a Till). PayTrack sends STK pushes with the platform's Daraja app, routing the money to the tenant's destination, and their backends receive signed webhooks with the outcome.

- Design and HA notes: [ARCHITECTURE.md](ARCHITECTURE.md)
- API contract (for the frontend and tenant integrations): [api/openapi.yaml](api/openapi.yaml)
- The original single-tenant demo is kept in [legacy/mpesastk](legacy/mpesastk)

## Run locally

1. Copy `.env.example` to `.env` and fill in the following.
   - `DATABASE_URL`: any Postgres 15 or later. SQLAlchemy-style `postgresql+asyncpg://` URLs are accepted.
   - `ENCRYPTION_KEY`: generate with `openssl rand -base64 32`.
   - `JWT_SECRET`: generate with `openssl rand -base64 48`.
   - `PUBLIC_CALLBACK_BASE_URL`: a public HTTPS URL that forwards to the callback role (`:9711`), for example `ngrok http 9711`. Daraja won't call `localhost`.
   - `MPESA_*`: the platform Daraja app (consumer key and secret, shortcode, passkey). It is used for every tenant's pushes.
   - `APP_BASE_URL`: the dashboard URL used in email links (`http://localhost:3009` in dev).
   - `RESEND_API_KEY` and `EMAIL_FROM`: transactional email. Without the key, emails (including verification links and sign-in codes) are printed to the log.
   - `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` (optional): Google sign-in. In Google Cloud Console, add `<dashboard origin>/api/v1/auth/google/callback` as an authorized redirect URI for each dashboard domain.
   - `COOKIE_SECURE=false` only for plain-http localhost.
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
   # dashboard sign-up → access token (the dashboard itself uses /api + an httpOnly refresh cookie)
   curl -XPOST localhost:9710/v1/auth/register -d '{"business_name":"Shop","email":"me@shop.co.ke","password":"a-long-password"}'
   # verify the email (link in the inbox, or in the log without RESEND_API_KEY), then
   # with the token: create an API key for the tenant backend, set the webhook URL
   curl -XPOST localhost:9710/v1/api-keys -H "Authorization: Bearer $JWT" -d '{"name":"backend"}'
   curl -XPUT  localhost:9710/v1/webhook  -H "Authorization: Bearer $JWT" -d '{"url":"https://shop.co.ke/hooks/paytrack"}'
   # with the API key: register a destination and charge a customer
   curl localhost:9710/v1/banks
   curl -XPOST localhost:9710/v1/channels -H "Authorization: Bearer $KEY" -d '{"name":"Savings","type":"bank","bank_code":"247247","account_number":"0123456789"}'
   curl -XPOST localhost:9710/v1/channels -H "Authorization: Bearer $KEY" -d '{"name":"Shop","type":"till","business_number":"123456"}'
   curl -XPOST localhost:9710/v1/payments -H "Authorization: Bearer $KEY" -H "Idempotency-Key: order-123" \
        -d '{"channel_id":"chn_…","amount":10,"phone":"0712345678"}'
   ```

## Google sign-in setup

In [Google Cloud Console](https://console.cloud.google.com/apis/credentials) → **APIs & Services → Credentials → OAuth 2.0 Client ID** (type *Web application*), add:

| Field | Values |
|---|---|
| Authorized JavaScript origins | `http://localhost:3009`<br>`https://paytrack.pesagrid.co.ke`<br>`https://paytrack.geninworld.com` |
| Authorized redirect URIs | `http://localhost:3009/api/v1/auth/google/callback`<br>`https://paytrack.pesagrid.co.ke/api/v1/auth/google/callback`<br>`https://paytrack.geninworld.com/api/v1/auth/google/callback` |

The redirect lands on the **dashboard** domain (its `/api` path is proxied to this service), so the session cookie is set first-party there. Don't add the API domain. Put the client ID and secret in `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`. For a new dashboard domain, add both entries above plus the origin in `CORS_ALLOWED_ORIGINS`.

## Roles

`paytrack <migrate|api|callback|worker|all>`: one image, with the role chosen per container. See ARCHITECTURE.md for what each role does.

| Role | Port | Health |
|---|---|---|
| api | `API_ADDR` (9710) | `/healthz`, `/readyz` |
| callback | `CALLBACK_ADDR` (9711) | `/healthz`, `/readyz` |
| worker | `WORKER_ADDR` (9712) | `/healthz`, `/readyz` |

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
