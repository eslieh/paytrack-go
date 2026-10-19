// Source of truth for the /docs page. Mirrors server/api/openapi.yaml and the
// server's validation rules; keep the two in sync when the API changes.

export const DASHBOARD_URL = "https://paytrack.pesagrid.co.ke";

// ------------------------------------------------------------------ examples

const T0 = "2026-10-15T09:24:11Z";
const T1 = "2026-10-15T09:24:38Z";

export const EX = {
  tenant: { id: "ten_01k7x2c9m4q8r6t3v5w7y9z1ab", name: "Acme Shop", created_at: "2026-10-01T08:00:00Z" },
  user: { id: "usr_01k7x2c9m5a1b2c3d4e5f6g7hj", email: "owner@acme.co.ke", role: "owner" },
  apiKey: {
    id: "key_01k7x3d1n2p3q4r5s6t7v8w9xy",
    name: "production backend",
    prefix: "ptk_9f3k2m",
    created_at: T0,
    last_used_at: null,
    revoked_at: null,
  },
  bank: { code: "247247", name: "Equity Bank" },
  channelBank: {
    id: "chn_01k7x4e2p3q4r5s6t7v8w9x0yz",
    name: "Main account",
    type: "bank",
    business_number: "247247",
    bank_name: "Equity Bank",
    account_number: "0123456789",
    active: true,
    created_at: T0,
  },
  channelPaybill: {
    id: "chn_01k7x4e9a8b7c6d5e4f3g2h1jk",
    name: "Utility PayBill",
    type: "paybill",
    business_number: "400200",
    account_number: "ACC001",
    active: true,
    created_at: T0,
  },
  channelTill: {
    id: "chn_01k7x4f0m1n2p3q4r5s6t7v8wx",
    name: "Shop till",
    type: "till",
    business_number: "123456",
    active: true,
    created_at: T0,
  },
  webhook: { url: "https://acme.co.ke/hooks/paytrack", signing_secret: "whsec_4c1f9a0e7b2d8c3f6a5e1b9d0c7f2a8e4b6d1c3f5a7e9b0d" },
};

EX.paymentPending = {
  id: "pay_01k7x5g3r4s5t6v7w8x9y0z1ab",
  channel_id: EX.channelBank.id,
  destination_type: "bank",
  business_number: "247247",
  bank_name: "Equity Bank",
  status: "pending",
  amount: 1500,
  currency: "KES",
  phone: "254712345678",
  account_reference: "0123456789",
  description: "Order 1042",
  metadata: { order_id: "1042" },
  merchant_request_id: "29115-34620561-1",
  checkout_request_id: "ws_CO_15102026122411734712345678",
  created_at: T0,
  updated_at: T0,
};

EX.paymentSucceeded = {
  ...EX.paymentPending,
  status: "succeeded",
  mpesa_receipt_number: "TJF3K8Q2XD",
  result_code: 0,
  result_desc: "The service request is processed successfully.",
  updated_at: T1,
  completed_at: T1,
};

EX.event = {
  id: "evt_01k7x5h8a9b0c1d2e3f4g5h6jk",
  type: "payment.succeeded",
  tenant_id: EX.tenant.id,
  created_at: T1,
  data: EX.paymentSucceeded,
};

EX.delivery = {
  id: "whd_01k7x5h8c0d1e2f3g4h5j6k7mn",
  event_id: EX.event.id,
  event_type: "payment.succeeded",
  payment_id: EX.paymentSucceeded.id,
  status: "delivered",
  attempts: 1,
  next_attempt_at: null,
  last_status_code: 200,
  delivered_at: "2026-10-15T09:24:39Z",
  created_at: T1,
  payload: EX.event,
};

const err = (code, message) => ({ error: { code, message, request_id: "req_01k7x5j2p3q4r5s6t7v8w9x0yz" } });

// ------------------------------------------------------------------ errors

export const ERROR_STATUSES = [
  ["200 / 201 / 202 / 204", "Success."],
  ["401 Unauthorized", "No API key, or the key is invalid or revoked."],
  ["404 Not Found", "The object doesn't exist or belongs to another account."],
  ["409 Conflict", "The request conflicts with current state (Idempotency-Key reused, delivery in flight)."],
  ["422 Unprocessable Entity", "The request was understood but a field is invalid. `error.code` says which rule failed."],
  ["500 Internal Server Error", "Something went wrong on our side. Safe to retry with the same Idempotency-Key."],
];

export const ERROR_CODES = [
  ["missing_token", 401, "No `Authorization: Bearer ptk_…` header."],
  ["invalid_api_key", 401, "API key is unknown or revoked."],
  ["idempotency_key_reused", 409, "The Idempotency-Key was already used with a different request body."],
  ["delivery_in_progress", 409, "The webhook delivery is being attempted right now; try again shortly."],
  ["payment_not_found", 404, "No payment with that ID on your account."],
  ["channel_not_found", "404 / 422", "No channel with that ID on your account (422 when referenced from a payment)."],
  ["delivery_not_found", 404, "No webhook delivery with that ID on your account."],
  ["invalid_json", 422, "Body isn't valid JSON or contains unknown fields."],
  ["idempotency_key_required", 422, "`Idempotency-Key` header missing or longer than 255 characters."],
  ["invalid_amount", 422, "Amount must be a whole number of KES from 1 to 250,000."],
  ["invalid_phone", 422, "Phone must be a Safaricom number, e.g. 0712345678 or 254712345678."],
  ["invalid_account_reference", 422, "1–20 characters: letters, digits, space and `-_/.`"],
  ["account_reference_required", 422, "The PayBill channel has no default account, so `account_reference` is required."],
  ["account_reference_mismatch", 422, "Bank channels always credit the channel's account; omit `account_reference`."],
  ["invalid_description", 422, "Description must be at most 13 characters."],
  ["invalid_metadata", 422, "At most 20 keys; keys ≤ 40 chars, values ≤ 500 chars."],
  ["channel_inactive", 422, "The channel has been deactivated."],
  ["invalid_channel_type", 422, "`type` must be `bank`, `paybill` or `till`."],
  ["invalid_bank_code", 422, "`bank_code` must be a code from `GET /v1/banks`."],
  ["account_number_required", 422, "Bank channels need the `account_number` to credit."],
  ["invalid_account_number", 422, "1–20 characters: letters, digits, space and `-_/.`"],
  ["invalid_business_number", 422, "PayBill / Till number must be 5–7 digits."],
  ["invalid_status", 422, "Unknown `status` filter value."],
  ["invalid_name", 422, "Name must be 1–100 characters."],
  ["invalid_webhook_url", 422, "Webhook URL must be an absolute http(s) URL without credentials."],
  ["internal_error", 500, "Unexpected server error. Quote the `request_id` when contacting support."],
];

// ------------------------------------------------------------------ objects

export const OBJECTS = [
  {
    id: "object-payment",
    name: "The Payment object",
    intro: "A Payment is one STK push to one customer. It records where the money was routed at the moment it was created, so later changes to the channel never rewrite history.",
    fields: [
      ["id", "string", "Unique identifier, prefixed `pay_`. IDs sort by creation time."],
      ["channel_id", "string", "The channel the payment was sent through."],
      ["destination_type", "enum", "`bank`, `paybill` or `till`. Snapshot of the channel type at creation."],
      ["business_number", "string", "The PayBill / Till the money was sent to. For bank channels, the bank's PayBill."],
      ["bank_name", "string", "Bank name for bank destinations. Omitted otherwise."],
      ["status", "enum", "`created`, `pending`, `succeeded`, `failed`, `cancelled` or `expired`. See Payment lifecycle."],
      ["amount", "integer", "Whole Kenyan shillings."],
      ["currency", "string", "Always `KES`."],
      ["phone", "string", "Customer number normalised to `2547XXXXXXXX` / `2541XXXXXXXX`."],
      ["account_reference", "string", "Account shown to the customer and credited at the destination."],
      ["description", "string", "Up to 13 characters shown on the M-Pesa prompt. Defaults to `Payment`."],
      ["metadata", "object", "Your key/value strings, returned untouched. Omitted when empty."],
      ["merchant_request_id", "string", "Safaricom's request ID. Present once Daraja accepted the push."],
      ["checkout_request_id", "string", "Safaricom's checkout ID. Useful when talking to Safaricom support."],
      ["mpesa_receipt_number", "string", "The M-Pesa receipt, e.g. `TJF3K8Q2XD`. Empty if the result was learned via a status query."],
      ["result_code", "integer", "Safaricom result code. `0` is success, `1032` is cancelled by the customer."],
      ["result_desc", "string", "Safaricom's human-readable result."],
      ["failure_reason", "string", "Why PayTrack failed or expired the payment, when there's no Safaricom result."],
      ["created_at", "timestamp", "RFC 3339, UTC."],
      ["updated_at", "timestamp", "RFC 3339, UTC."],
      ["completed_at", "timestamp | null", "When the payment reached its current final state."],
    ],
    example: EX.paymentSucceeded,
  },
  {
    id: "object-channel",
    name: "The Channel object",
    intro: "A Channel is a destination for money. PayTrack sends every STK push from its own Daraja app; the channel decides which bank account, PayBill or Till is credited.",
    fields: [
      ["id", "string", "Unique identifier, prefixed `chn_`."],
      ["name", "string", "Your label for the channel."],
      ["type", "enum", "`bank`, `paybill` or `till`."],
      ["business_number", "string", "The PayBill or Till. For `bank`, the bank's PayBill from `/v1/banks`."],
      ["bank_name", "string", "Only for `bank` channels."],
      ["account_number", "string", "`bank`: the account credited. `paybill`: optional default account reference. Omitted for `till`."],
      ["active", "boolean", "`false` once deactivated. Inactive channels reject new payments."],
      ["created_at", "timestamp", "RFC 3339, UTC."],
    ],
    example: EX.channelBank,
  },
  {
    id: "object-event",
    name: "The Event object",
    intro: "Events are what PayTrack POSTs to your webhook URL. `data` is the full Payment at the time of the event.",
    fields: [
      ["id", "string", "Unique identifier, prefixed `evt_`. Dedupe on this."],
      ["type", "enum", "`payment.succeeded`, `payment.failed`, `payment.cancelled` or `payment.expired`."],
      ["tenant_id", "string", "Your account ID."],
      ["created_at", "timestamp", "When the event happened."],
      ["data", "Payment", "Snapshot of the payment."],
    ],
    example: EX.event,
  },
  {
    id: "object-delivery",
    name: "The Delivery object",
    intro: "A Delivery tracks sending one Event to your webhook URL, including every retry.",
    fields: [
      ["id", "string", "Unique identifier, prefixed `whd_`. Sent as `PayTrack-Delivery-Id`."],
      ["event_id", "string", "The event being delivered."],
      ["event_type", "string", "Same as the event's `type`."],
      ["payment_id", "string", "The payment the event is about."],
      ["status", "enum", "`pending`, `delivering`, `delivered`, `failed` (retries exhausted) or `skipped` (no webhook URL was set when the event happened; nothing was sent, and you can redeliver it after setting one)."],
      ["attempts", "integer", "Attempts made so far. Reset to 0 by a redelivery."],
      ["next_attempt_at", "timestamp | null", "When the next retry is due."],
      ["last_error", "string", "Last transport error or `endpoint returned HTTP 5xx`."],
      ["last_status_code", "integer", "HTTP status your endpoint returned last time."],
      ["delivered_at", "timestamp | null", "When a 2xx was received."],
      ["created_at", "timestamp", "RFC 3339, UTC."],
      ["payload", "Event", "The exact event body sent."],
    ],
    example: EX.delivery,
  },
];

// ------------------------------------------------------------------ endpoints
//
// param: [name, type, required, description]
// request: what the samples send. `auth`: "key" | "none".

export const ENDPOINT_GROUPS = [
  {
    id: "payments",
    title: "Payments",
    intro: "Charge a customer with an M-Pesa STK push, then follow the payment until it settles.",
    endpoints: [
      {
        id: "create-payment",
        title: "Create a payment",
        method: "POST",
        path: "/v1/payments",
        auth: "key",
        description:
          "Sends an STK push to the customer's phone. Returns `201` with status `pending` once Safaricom accepts the push, `failed` if Safaricom rejected it, or `created` if Safaricom's response was ambiguous. In every case the final outcome arrives by webhook. Retrying with the same `Idempotency-Key` and body returns the original payment with `200` and the header `Idempotent-Replayed: true`.",
        headers: [["Idempotency-Key", "string", true, "Unique per charge, up to 255 characters. Use your order ID or a UUID."]],
        body: [
          ["channel_id", "string", true, "The channel to route the money to."],
          ["amount", "integer", true, "Whole KES, 1 – 250,000."],
          ["phone", "string", true, "Safaricom number in any common format: `0712345678`, `712345678`, `+254712345678`, `254712345678`. `01…` numbers work too."],
          ["account_reference", "string", false, "1–20 chars. **bank:** omit. **paybill:** overrides the channel default; required if it has none. **till:** optional, defaults to `Till Payment`."],
          ["description", "string", false, "Up to 13 characters shown on the prompt. Defaults to `Payment`."],
          ["metadata", "object", false, "Up to 20 string key/value pairs (keys ≤ 40, values ≤ 500 chars). Returned on the payment and in webhooks."],
        ],
        request: {
          headers: { "Idempotency-Key": "order-1042" },
          body: { channel_id: EX.channelBank.id, amount: 1500, phone: "0712345678", description: "Order 1042", metadata: { order_id: "1042" } },
        },
        responses: [
          { status: "201 Created", body: EX.paymentPending },
          { status: "200 OK (replayed)", body: EX.paymentPending },
          { status: "422 Unprocessable Entity", body: err("invalid_phone", "phone must be a Safaricom number, e.g. 0712345678 or 254712345678") },
          { status: "409 Conflict", body: err("idempotency_key_reused", "Idempotency-Key was already used with a different request body") },
        ],
      },
      {
        id: "retrieve-payment",
        title: "Retrieve a payment",
        method: "GET",
        path: "/v1/payments/{id}",
        auth: "key",
        description: "Fetches the current state of a payment. Prefer webhooks for completion; poll sparingly (every few seconds at most) if you must.",
        pathParams: [["id", "string", true, "The payment ID, e.g. `pay_01k7…`."]],
        request: { path: { id: EX.paymentSucceeded.id } },
        responses: [
          { status: "200 OK", body: EX.paymentSucceeded },
          { status: "404 Not Found", body: err("payment_not_found", "payment not found") },
        ],
      },
      {
        id: "check-payment-status",
        title: "Check a payment's status with M-Pesa",
        method: "POST",
        path: "/v1/payments/{id}/check-status",
        auth: "key",
        description:
          "Asks M-Pesa for the result right now instead of waiting for the callback. PayTrack already does this automatically when a callback is late (45s, 1m, 2m, 5m, 10m, 20m), so you rarely need it. A result is applied exactly like a callback: the payment updates and your webhook is sent. Works for `pending` payments, and for `expired` ones (which can still become `succeeded`). Limited to 6 checks per payment per minute. `outcome` is `updated`, `still_processing`, `unchanged`, `final` or `not_queryable`. Payments confirmed this way have no `mpesa_receipt_number`, because M-Pesa's status query doesn't return one.",
        pathParams: [["id", "string", true, "The payment ID."]],
        request: { path: { id: EX.paymentSucceeded.id } },
        responses: [
          { status: "200 OK", body: { payment: EX.paymentSucceeded, outcome: "updated", message: "M-Pesa returned a result; the payment was updated and your webhook is on its way." } },
          { status: "429 Too Many Requests", body: err("rate_limited", "this payment was checked recently; try again in a minute") },
          { status: "503 Service Unavailable", body: err("mpesa_unavailable", "M-Pesa didn't answer the status check; try again shortly") },
        ],
      },
      {
        id: "list-payments",
        title: "List payments",
        method: "GET",
        path: "/v1/payments",
        auth: "key",
        description: "Returns your payments, newest first, with cursor pagination.",
        query: [
          ["status", "enum", false, "`created`, `pending`, `succeeded`, `failed`, `cancelled` or `expired`."],
          ["channel_id", "string", false, "Only payments sent through this channel."],
          ["phone", "string", false, "Only payments to this phone. Any accepted format."],
          ["limit", "integer", false, "1–100. Default 25."],
          ["cursor", "string", false, "`next_cursor` from the previous page."],
        ],
        request: { query: { status: "succeeded", limit: 2 } },
        responses: [
          { status: "200 OK", body: { data: [EX.paymentSucceeded], has_more: true, next_cursor: EX.paymentSucceeded.id } },
        ],
      },
    ],
  },
  {
    id: "channels",
    title: "Channels",
    intro: "Channels are where money lands. Create one per bank account, PayBill or Till you collect into.",
    endpoints: [
      {
        id: "list-banks",
        title: "List banks",
        method: "GET",
        path: "/v1/banks",
        auth: "none",
        description: "Banks you can use as a `bank` channel destination. Public, no authentication needed. `code` is the bank's PayBill.",
        request: {},
        responses: [
          {
            status: "200 OK",
            body: {
              data: [
                { code: "522522", name: "KCB Bank" },
                { code: "247247", name: "Equity Bank" },
                { code: "400200", name: "Co-operative Bank" },
                { code: "600100", name: "Stanbic Bank" },
                { code: "880100", name: "NCBA Bank" },
                { code: "972900", name: "Bank of Africa" },
                { code: "111777", name: "ABC Bank" },
                { code: "902363", name: "Bank of Baroda" },
              ],
            },
          },
        ],
      },
      {
        id: "create-channel",
        title: "Create a channel",
        method: "POST",
        path: "/v1/channels",
        auth: "key",
        description: "Registers a destination. Which fields are required depends on `type`.",
        body: [
          ["name", "string", true, "Your label, 1–100 characters."],
          ["type", "enum", true, "`bank`, `paybill` or `till`."],
          ["bank_code", "string", false, "**Required for `bank`.** A `code` from List banks."],
          ["business_number", "string", false, "**Required for `paybill` and `till`.** 5–7 digits."],
          ["account_number", "string", false, "**Required for `bank`** (the account credited). Optional default for `paybill`. Ignored for `till`. 1–20 chars."],
        ],
        variants: [
          { label: "Bank", body: { name: "Main account", type: "bank", bank_code: "247247", account_number: "0123456789" }, response: EX.channelBank },
          { label: "PayBill", body: { name: "Utility PayBill", type: "paybill", business_number: "400200", account_number: "ACC001" }, response: EX.channelPaybill },
          { label: "Till", body: { name: "Shop till", type: "till", business_number: "123456" }, response: EX.channelTill },
        ],
        request: { body: { name: "Main account", type: "bank", bank_code: "247247", account_number: "0123456789" } },
        responses: [
          { status: "201 Created", body: EX.channelBank },
          { status: "422 Unprocessable Entity", body: err("invalid_bank_code", "bank_code must be one of the codes from GET /v1/banks") },
        ],
      },
      {
        id: "retrieve-channel",
        title: "Retrieve a channel",
        method: "GET",
        path: "/v1/channels/{id}",
        auth: "key",
        description: "Fetches one channel.",
        pathParams: [["id", "string", true, "The channel ID."]],
        request: { path: { id: EX.channelBank.id } },
        responses: [
          { status: "200 OK", body: EX.channelBank },
          { status: "404 Not Found", body: err("channel_not_found", "channel not found") },
        ],
      },
      {
        id: "list-channels",
        title: "List channels",
        method: "GET",
        path: "/v1/channels",
        auth: "key",
        description: "Returns all your channels, including deactivated ones. Not paginated.",
        request: {},
        responses: [{ status: "200 OK", body: { data: [EX.channelBank, EX.channelTill] } }],
      },
      {
        id: "deactivate-channel",
        title: "Deactivate a channel",
        method: "DELETE",
        path: "/v1/channels/{id}",
        auth: "key",
        description: "New payments on the channel are rejected with `channel_inactive`. Payments already in flight still complete. Deactivation is permanent; create a new channel to reuse the destination.",
        pathParams: [["id", "string", true, "The channel ID."]],
        request: { path: { id: EX.channelBank.id } },
        responses: [{ status: "204 No Content", body: null }],
      },
    ],
  },
  {
    id: "webhook-endpoints",
    title: "Webhooks",
    intro: "Configure where events are sent, rotate the signing secret, and inspect or replay deliveries.",
    endpoints: [
      {
        id: "get-webhook",
        title: "Retrieve webhook settings",
        method: "GET",
        path: "/v1/webhook",
        auth: "key",
        description: "Returns the webhook URL (empty until set) and the signing secret. To rotate the secret, use the dashboard.",
        request: {},
        responses: [{ status: "200 OK", body: EX.webhook }],
      },
      {
        id: "set-webhook",
        title: "Set the webhook URL",
        method: "PUT",
        path: "/v1/webhook",
        auth: "key",
        description: "Sets the URL events are POSTed to. Takes effect for the next attempt, including retries of earlier events.",
        body: [["url", "string", true, "Absolute `https://` URL (plain `http://` is accepted but not recommended). No credentials in the URL."]],
        request: { body: { url: EX.webhook.url } },
        responses: [
          { status: "200 OK", body: EX.webhook },
          { status: "422 Unprocessable Entity", body: err("invalid_webhook_url", "webhook url must be an absolute http(s) URL without credentials") },
        ],
      },
      {
        id: "list-deliveries",
        title: "List deliveries",
        method: "GET",
        path: "/v1/webhook/deliveries",
        auth: "key",
        description: "Every attempt to send an event to you, newest first, with cursor pagination.",
        query: [
          ["payment_id", "string", false, "Only deliveries for this payment."],
          ["status", "enum", false, "`pending`, `delivering`, `delivered`, `failed` or `skipped`."],
          ["limit", "integer", false, "1–100. Default 25."],
          ["cursor", "string", false, "`next_cursor` from the previous page."],
        ],
        request: { query: { payment_id: EX.paymentSucceeded.id } },
        responses: [{ status: "200 OK", body: { data: [EX.delivery], has_more: false } }],
      },
      {
        id: "redeliver",
        title: "Redeliver an event",
        method: "POST",
        path: "/v1/webhook/deliveries/{id}/redeliver",
        auth: "key",
        description: "Queues the delivery again immediately with a fresh retry budget. The same event ID is sent, so your deduplication keeps it safe.",
        pathParams: [["id", "string", true, "The delivery ID (`whd_…`)."]],
        request: { path: { id: EX.delivery.id } },
        responses: [
          { status: "202 Accepted", body: { ...EX.delivery, status: "pending", attempts: 0, next_attempt_at: "2026-10-15T10:02:00Z" } },
          { status: "409 Conflict", body: err("delivery_in_progress", "delivery is currently being attempted") },
        ],
      },
    ],
  },
  {
    id: "account",
    title: "Account",
    intro: "Check which account an API key belongs to.",
    endpoints: [
      {
        id: "me",
        title: "Verify your API key",
        method: "GET",
        path: "/v1/me",
        auth: "key",
        description: "Returns the account the API key belongs to. Call it at startup or in a health check to confirm the key is valid.",
        request: {},
        responses: [{ status: "200 OK", body: { tenant: EX.tenant, user: null, auth_kind: "api_key" } }],
      },
    ],
  },
];

export const WEBHOOK_RETRY_SCHEDULE = ["10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "4h", "8h"];
