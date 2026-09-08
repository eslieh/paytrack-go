# mpesastk

A small Go web app that initiates M-Pesa **STK Push** (Lipa Na M-Pesa Online) payment prompts and routes them to one of three destination types: a bank account (via the bank's PayBill), an arbitrary business PayBill, or a merchant Till (Buy Goods).

## How it works

1. **Web UI** (`main.go`) — a single HTML page (rendered server-side via `html/template`, no frontend build step) with a form for phone number, amount, and a payment destination. Selecting "Bank" shows a dropdown of curated banks; "Paybill" or "Till" show a raw number field instead.
2. **Submit** — the form POSTs JSON to `/stkpush`. The server validates the amount and payment type, then calls into the M-Pesa client.
3. **M-Pesa client** (`mpesa.go`):
   - Requests an OAuth access token from Safaricom's Daraja API using the app's consumer key/secret (`getAccessToken`). Tokens are fetched fresh per request rather than cached.
   - Normalizes and validates input: phone numbers (`07xx`, `7xx`, `+254xx`, `254xx` → `2547xxxxxxxx`), PayBill/Till numbers (5-7 digits), and account references (max 12 chars, Daraja's limit).
   - Builds the STK Push request. Bank and Paybill destinations use `CustomerPayBillOnline`; Till uses `CustomerBuyGoodsOnline`. For banks, `PartyB` is the bank's own PayBill shortcode and the customer-supplied account number is passed through as `AccountReference`.
   - Signs the request with a `Password` (base64 of `Shortcode + Passkey + Timestamp`, as Daraja requires) and POSTs to `/mpesa/stkpush/v1/processrequest`.
   - Returns the `CheckoutRequestID`/`MerchantRequestID` and customer-facing message back to the browser, which displays it as success or error.
4. **Async callback** — Safaricom calls back to `POST /mpesa/callback` once the customer approves or cancels the prompt on their phone. This must be a publicly reachable HTTPS URL (e.g. via `ngrok` in development) matching `MPESA_CALLBACK_URL`. The handler currently just logs the payload and acknowledges it.

Configuration (`config.go`) is loaded entirely from environment variables at startup; the app refuses to start if required Daraja credentials are missing. `MPESA_ENV=production` switches the Daraja base URL from sandbox to live.

## Routes

| Method | Path              | Purpose                                   |
|--------|-------------------|--------------------------------------------|
| GET    | `/`               | Serves the payment form                    |
| POST   | `/stkpush`        | Validates input and triggers an STK push   |
| POST   | `/mpesa/callback` | Receives Daraja's async payment result     |

## Built with

- **Go** (module `mpesastk`, go 1.26.1) — no third-party dependencies, just the standard library (`net/http`, `html/template`, `encoding/json`, `regexp`).
- **Safaricom Daraja API** — sandbox (`sandbox.safaricom.co.ke`) or production (`api.safaricom.co.ke`), specifically the OAuth and Lipa Na M-Pesa Online (STK Push) endpoints.

## Running it

1. Copy `.env.example` to `.env` and fill in your Daraja app credentials:
   ```
   MPESA_ENV=sandbox
   MPESA_CONSUMER_KEY=
   MPESA_CONSUMER_SECRET=
   MPESA_SHORTCODE=
   MPESA_PASSKEY=
   MPESA_CALLBACK_URL=https://your-public-url/mpesa/callback
   LISTEN_ADDR=:8080
   ```
2. If testing locally, expose the server with a tunnel (e.g. `ngrok http 8080`) and set `MPESA_CALLBACK_URL` to the tunnel's `/mpesa/callback` URL — Daraja will not call back to `localhost`.
3. Run the server:
   ```
   go run .
   ```
4. Open `http://localhost:8080`, fill in a Safaricom test number, amount, and destination, and submit. Watch the server logs for the callback once the STK prompt is actioned on the test phone.

## Use cases

- **Collections routing** — a single integration point for accepting M-Pesa payments that need to land in different places: a customer's own bank account, a business PayBill, or a merchant Till, without maintaining separate integrations for each.
- **Bank deposits over M-Pesa** — letting a user pay into a specific bank account (e.g. loan repayment, savings top-up) by picking the bank and entering their account number, using the bank's PayBill as the underlying rails.
- **Prototype/reference implementation** — a minimal, dependency-free example of driving Safaricom's Daraja STK Push API from Go, useful as a starting point before adding persistence, auth, transaction status polling, or a production-grade callback handler.

## Notes / limitations

- This is a lightweight/demo implementation: there's no database, no persistence of transaction state, and the callback handler only logs incoming results rather than reconciling them against an initiated request.
- OAuth tokens are not cached; each STK push fetches a new token, which is wasteful under load but simpler for a low-volume demo.
- The bank list in `mpesa.go` (`Banks`) is a hardcoded sample of Kenyan banks' PayBill shortcodes — verify and update these against current Safaricom/bank documentation before using in production.
