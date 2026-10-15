// Thin client for the PayTrack API (see server/api/openapi.yaml).
import { clearSession, getToken } from "./storage";

const BASE_URL = (process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/+$/, "");

// ApiError carries the server's error envelope: {"error": {code, message, request_id}}.
export class ApiError extends Error {
  constructor(status, code, message, requestId) {
    super(message);
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

export async function apiFetch(path, { body, headers, auth = true, ...options } = {}) {
  const token = auth ? getToken() : null;
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(headers || {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return null;
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }

  if (!res.ok) {
    const err = data && data.error;
    if (res.status === 401 && token) {
      // Session expired or revoked: drop it so the dashboard guard sends the user to sign in.
      clearSession();
      if (typeof window !== "undefined") window.dispatchEvent(new Event("paytrack:unauthorized"));
    }
    throw new ApiError(
      res.status,
      err?.code || "http_error",
      err?.message || (typeof data === "string" && data) || `Request failed (${res.status})`,
      err?.request_id || res.headers.get("X-Request-Id") || undefined,
    );
  }
  return data;
}

export function getBaseUrl() {
  return BASE_URL;
}

function qs(params = {}) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== "") q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

const enc = encodeURIComponent;

export const api = {
  auth: {
    register: ({ business_name, email, password }) =>
      apiFetch("/v1/auth/register", { method: "POST", auth: false, body: { business_name, email, password } }),
    login: ({ email, password }) =>
      apiFetch("/v1/auth/login", { method: "POST", auth: false, body: { email, password } }),
    me: () => apiFetch("/v1/me"),
  },

  apiKeys: {
    list: () => apiFetch("/v1/api-keys"),
    create: (name) => apiFetch("/v1/api-keys", { method: "POST", body: { name } }),
    revoke: (id) => apiFetch(`/v1/api-keys/${enc(id)}`, { method: "DELETE" }),
  },

  banks: {
    list: () => apiFetch("/v1/banks", { auth: false }),
  },

  channels: {
    list: () => apiFetch("/v1/channels"),
    get: (id) => apiFetch(`/v1/channels/${enc(id)}`),
    create: (channel) => apiFetch("/v1/channels", { method: "POST", body: channel }),
    deactivate: (id) => apiFetch(`/v1/channels/${enc(id)}`, { method: "DELETE" }),
  },

  webhook: {
    get: () => apiFetch("/v1/webhook"),
    setUrl: (url) => apiFetch("/v1/webhook", { method: "PUT", body: { url } }),
    rotateSecret: () => apiFetch("/v1/webhook/rotate-secret", { method: "POST" }),
    deliveries: ({ payment_id, status, cursor, limit } = {}) =>
      apiFetch(`/v1/webhook/deliveries${qs({ payment_id, status, cursor, limit })}`),
    redeliver: (id) => apiFetch(`/v1/webhook/deliveries/${enc(id)}/redeliver`, { method: "POST" }),
  },

  payments: {
    list: ({ status, channel_id, phone, cursor, limit } = {}) =>
      apiFetch(`/v1/payments${qs({ status, channel_id, phone, cursor, limit })}`),
    get: (id) => apiFetch(`/v1/payments/${enc(id)}`),
    // idempotencyKey must be stable across retries of the same charge.
    create: (payment, idempotencyKey) =>
      apiFetch("/v1/payments", { method: "POST", body: payment, headers: { "Idempotency-Key": idempotencyKey } }),
  },
};
