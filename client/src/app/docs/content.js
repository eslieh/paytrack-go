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
  ["401 Unauthorized", "No token, or the API key / session is invalid, revoked or expired."],
  ["403 Forbidden", "An API key was used on an endpoint that needs a dashboard session."],
  ["404 Not Found", "The object doesn't exist or belongs to another account."],
  ["409 Conflict", "The request conflicts with current state (email taken, Idempotency-Key reused, delivery in flight)."],
  ["422 Unprocessable Entity", "The request was understood but a field is invalid. `error.code` says which rule failed."],
  ["500 Internal Server Error", "Something went wrong on our side. Safe to retry with the same Idempotency-Key."],
];

export const ERROR_CODES = [
  ["missing_token", 401, "No `Authorization: Bearer …` header."],
  ["invalid_api_key", 401, "API key is unknown or revoked."],
  ["invalid_token", 401, "Dashboard session token is invalid or expired."],
  ["invalid_credentials", 401, "Email or password is incorrect."],
  ["user_session_required", 403, "Endpoint requires a dashboard session; API keys aren't accepted."],
  ["email_taken", 409, "An account with this email already exists."],
  ["idempotency_key_reused", 409, "The Idempotency-Key was already used with a different request body."],
  ["delivery_in_progress", 409, "The webhook delivery is being attempted right now; try again shortly."],
  ["payment_not_found", 404, "No payment with that ID on your account."],
  ["channel_not_found", "404 / 422", "No channel with that ID on your account (422 when referenced from a payment)."],
  ["api_key_not_found", 404, "No API key with that ID on your account."],
  ["delivery_not_found", 404, "No webhook delivery with that ID on your account."],
  ["invalid_json", 422, "Body isn't valid JSON or contains unknown fields."],
  ["idempotency_key_required", 422, "`Idempotency-Key` header missing or longer than 255 characters."],
  ["invalid_amount", 422, "Amount must be a whole number of KES from 1 to 250,000."],
  ["invalid_phone", 422, "Phone must be a Safaricom number, e.g. 0712345678 or 254712345678."],
  ["invalid_account_reference", 422, "1–12 characters: letters, digits, space and `-_/.`"],
  ["account_reference_required", 422, "The PayBill channel has no default account, so `account_reference` is required."],
  ["account_reference_mismatch", 422, "Bank channels always credit the channel's account; omit `account_reference`."],
  ["invalid_description", 422, "Description must be at most 13 characters."],
  ["invalid_metadata", 422, "At most 20 keys; keys ≤ 40 chars, values ≤ 500 chars."],
  ["channel_inactive", 422, "The channel has been deactivated."],
  ["invalid_channel_type", 422, "`type` must be `bank`, `paybill` or `till`."],
  ["invalid_bank_code", 422, "`bank_code` must be a code from `GET /v1/banks`."],
  ["account_number_required", 422, "Bank channels need the `account_number` to credit."],
  ["invalid_account_number", 422, "1–12 characters: letters, digits, space and `-_/.`"],
  ["invalid_business_number", 422, "PayBill / Till number must be 5–7 digits."],
  ["invalid_status", 422, "Unknown `status` filter value."],
  ["invalid_email", 422, "Email address is invalid."],
  ["weak_password", 422, "Password must be 10–72 characters."],
  ["invalid_business_name", 422, "Business name must be 1–100 characters."],
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
      ["status", "enum", "`pending`, `delivering`, `delivered` or `failed` (retries exhausted)."],
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
  {
    id: "object-api-key",
    name: "The API Key object",
    intro: "API keys authenticate your backend. The secret is returned exactly once, in the create response.",
    fields: [
      ["id", "string", "Unique identifier, prefixed `key_`."],
      ["name", "string", "Your label."],
      ["prefix", "string", "First characters of the key, e.g. `ptk_9f3k2m`, to recognise it later."],
      ["key", "string", "The full secret. **Only present in the create response.**"],
      ["created_at", "timestamp", "RFC 3339, UTC."],
      ["last_used_at", "timestamp | null", "Last successful authentication."],
      ["revoked_at", "timestamp | null", "Set once revoked."],
    ],
    example: { ...EX.apiKey, key: "ptk_9f3k2m7q4x8d1v6b3n0c5z2w9h4j7k1e8r5t2y6u3p0s" },
  },
];

// ------------------------------------------------------------------ endpoints
//
// param: [name, type, required, description]
// request: what the samples send. `auth`: "key" | "session" | "none".

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
          ["account_reference", "string", false, "1–12 chars. **bank:** omit. **paybill:** overrides the channel default; required if it has none. **till:** optional, defaults to `Till Payment`."],
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
          ["account_number", "string", false, "**Required for `bank`** (the account credited). Optional default for `paybill`. Ignored for `till`. 1–12 chars."],
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
        description: "Returns the webhook URL (empty until set) and the signing secret.",
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
        id: "rotate-secret",
        title: "Rotate the signing secret",
        method: "POST",
        path: "/v1/webhook/rotate-secret",
        auth: "session",
        description: "Generates a new signing secret. The old one stops working immediately, including for retries, so deploy the new secret right away. Dashboard session only.",
        request: {},
        responses: [
          { status: "200 OK", body: { ...EX.webhook, signing_secret: "whsec_8d2b6f1a9c4e7b0d3f5a2c8e1b6d9f0a7c3e5b1d8f2a4c6e" } },
          { status: "403 Forbidden", body: err("user_session_required", "this endpoint requires a dashboard session") },
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
          ["status", "enum", false, "`pending`, `delivering`, `delivered` or `failed`."],
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
    title: "Account & API keys",
    intro: "Identify the caller and manage API keys. Key management needs a dashboard session.",
    endpoints: [
      {
        id: "me",
        title: "Retrieve the caller",
        method: "GET",
        path: "/v1/me",
        auth: "key",
        description: "Returns the account the token belongs to. `user` is `null` when called with an API key. Handy for verifying a key at startup.",
        request: {},
        responses: [{ status: "200 OK", body: { tenant: EX.tenant, user: null, auth_kind: "api_key" } }],
      },
      {
        id: "create-key",
        title: "Create an API key",
        method: "POST",
        path: "/v1/api-keys",
        auth: "session",
        description: "Creates a key. The `key` field is returned only in this response, so store it immediately. Dashboard session only.",
        body: [["name", "string", true, "Your label, 1–100 characters."]],
        request: { body: { name: "production backend" } },
        responses: [{ status: "201 Created", body: OBJECTS_KEY_EXAMPLE() }],
      },
      {
        id: "list-keys",
        title: "List API keys",
        method: "GET",
        path: "/v1/api-keys",
        auth: "session",
        description: "All keys, including revoked ones. Secrets are never returned. Dashboard session only.",
        request: {},
        responses: [{ status: "200 OK", body: { data: [EX.apiKey] } }],
      },
      {
        id: "revoke-key",
        title: "Revoke an API key",
        method: "DELETE",
        path: "/v1/api-keys/{id}",
        auth: "session",
        description: "Requests with the key fail with `invalid_api_key` immediately. Irreversible. Dashboard session only.",
        pathParams: [["id", "string", true, "The key ID (`key_…`)."]],
        request: { path: { id: EX.apiKey.id } },
        responses: [
          { status: "204 No Content", body: null },
          { status: "404 Not Found", body: err("api_key_not_found", "API key not found") },
        ],
      },
      {
        id: "register",
        title: "Register",
        method: "POST",
        path: "/v1/auth/register",
        auth: "none",
        description: "Creates an account and its owner user and returns a session. Used by the dashboard; most integrations never call it.",
        body: [
          ["business_name", "string", true, "1–100 characters."],
          ["email", "string", true, "Owner's email."],
          ["password", "string", true, "10–72 characters."],
        ],
        request: { body: { business_name: "Acme Shop", email: "owner@acme.co.ke", password: "correct-horse-battery" } },
        responses: [
          { status: "201 Created", body: { token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…", expires_at: "2026-10-15T21:24:11Z", user: EX.user, tenant: EX.tenant } },
          { status: "409 Conflict", body: err("email_taken", "an account with this email already exists") },
        ],
      },
      {
        id: "login",
        title: "Log in",
        method: "POST",
        path: "/v1/auth/login",
        auth: "none",
        description: "Exchanges email and password for a session token (valid 12 hours by default).",
        body: [
          ["email", "string", true, "Account email."],
          ["password", "string", true, "Account password."],
        ],
        request: { body: { email: "owner@acme.co.ke", password: "correct-horse-battery" } },
        responses: [
          { status: "200 OK", body: { token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…", expires_at: "2026-10-15T21:24:11Z", user: EX.user, tenant: EX.tenant } },
          { status: "401 Unauthorized", body: err("invalid_credentials", "email or password is incorrect") },
        ],
      },
    ],
  },
];

function OBJECTS_KEY_EXAMPLE() {
  return { ...EX.apiKey, key: "ptk_9f3k2m7q4x8d1v6b3n0c5z2w9h4j7k1e8r5t2y6u3p0s" };
}

export const WEBHOOK_RETRY_SCHEDULE = ["10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "4h", "8h"];
