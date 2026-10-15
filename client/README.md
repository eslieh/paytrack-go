# PayTrack dashboard

The Next.js dashboard and marketing site for PayTrack. It talks to the Go API in [`../server`](../server). The API contract is [`server/api/openapi.yaml`](../server/api/openapi.yaml).

## Run locally

1. Start the server (see [server/README.md](../server/README.md)). The API listens on `:8080` by default.
2. Let the dashboard origin through CORS. In `server/.env`, set:
   ```sh
   CORS_ALLOWED_ORIGINS=http://localhost:3009
   ```
3. Point the client at the API:
   ```sh
   cp .env.example .env.local   # NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
   ```
4. Run the dev server:
   ```sh
   npm install
   npm run dev                  # http://localhost:3009
   ```

## Pages

| Route | What it does | API |
|---|---|---|
| `/` | Landing page | none |
| `/auth` | Sign in (supports `?next=/path`) | `POST /v1/auth/login` |
| `/auth/register` | Create a business and its owner user | `POST /v1/auth/register` |
| `/dashboard` | Overview: business info, summary tiles, setup checklist, recent payments | `GET /v1/me`, `/v1/channels`, `/v1/webhook`, `/v1/api-keys`, `/v1/payments` |
| `/dashboard/payments` | Filterable payment list, payment detail (`?id=pay_…`) with its webhook deliveries, and a form to send a test STK push. The account reference field follows the channel type (see below) | `GET/POST /v1/payments`, `GET /v1/payments/{id}`, `GET /v1/webhook/deliveries?payment_id=` |
| `/dashboard/channels` | Add and deactivate payment destinations: a bank account (bank picked from `/v1/banks`), a PayBill (with an optional default account) or a Till | `GET /v1/banks`, `GET/POST /v1/channels`, `DELETE /v1/channels/{id}` |
| `/dashboard/webhooks` | Webhook URL, signing secret (reveal, copy, rotate), delivery log with redelivery | `GET/PUT /v1/webhook`, `POST /v1/webhook/rotate-secret`, `GET /v1/webhook/deliveries`, `POST /v1/webhook/deliveries/{id}/redeliver` |
| `/dashboard/api-keys` | Create named API keys (the full key is shown once) and revoke them | `GET/POST /v1/api-keys`, `DELETE /v1/api-keys/{id}` |
| `/receipt?id=pay_…` | Printable receipt for a succeeded payment | `GET /v1/payments/{id}` |

`/dashboard/wallet` redirects to `/dashboard/payments`. The Go backend has no wallet, because payments settle straight to the tenant's bank account, PayBill or Till.

## Code layout

```
src/
  lib/api.js                 fetch wrapper + typed `api.*` methods for every endpoint
  lib/storage.js             session persistence (localStorage)
  lib/format.js              currency/date/status helpers, idempotency key generator
  context/AuthContext.jsx    session state: login, signup, logout
  components/DashboardShell  auth guard + nav for every /dashboard page
  components/ui.jsx          StatusBadge, Modal, Field, KV, CopyButton, ErrorText
  app/…                      pages (see table above)
```

### Calling the API

Use the methods on `api` rather than raw `fetch`:

```js
import { api, ApiError } from "@/lib/api";

const { data, has_more, next_cursor } = await api.payments.list({ status: "succeeded", limit: 25 });
const more = await api.payments.list({ status: "succeeded", cursor: next_cursor });

try {
  await api.payments.create({ channel_id, amount: 10, phone: "0712345678" }, idempotencyKey);
} catch (e) {
  if (e instanceof ApiError) console.log(e.status, e.code, e.message, e.requestId);
}
```

- **Auth.** `apiFetch` sends the session JWT as `Authorization: Bearer …`. A 401 clears the session, and the dashboard shell redirects to `/auth`. Sessions also expire client-side at `expires_at` (server `JWT_TTL`, 12h by default).
- **Errors.** The server returns `{"error": {"code", "message", "request_id"}}`. These are surfaced as `ApiError`, and `ErrorText` shows the request ID for support.
- **Pagination.** List endpoints return `{data, has_more, next_cursor}`, newest first. Pass `cursor` to load older pages.
- **Idempotency.** `POST /v1/payments` needs an `Idempotency-Key`. The STK push form generates one per form, so resubmitting after a network error replays the original payment instead of charging twice.
- **Channels and `account_reference`.** STK pushes always go through the platform's Daraja app, and a channel only says where the money is routed. The rules for `account_reference` depend on the channel type:
  - **bank:** omit it. The channel's `account_number` is always used, and the form shows it read-only.
  - **paybill:** optional if the channel has a default `account_number`, which it overrides. Required otherwise.
  - **till:** optional.
- **User-session-only endpoints.** API keys and secret rotation reject API-key auth with 403. The dashboard always uses a user session, so this only matters for tenant backends.

## Build

```sh
npm run build && npm start
```
