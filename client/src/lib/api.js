// Thin client for the PayTrack API (see server/api/openapi.yaml).
// Dashboard requests go through the Next.js /api proxy with cookies; an
// expired access token is refreshed once, transparently.
import { clearSession, getSession, getToken, setSession } from "./storage";

const API = "/api";
const PUBLIC_API_URL = (process.env.NEXT_PUBLIC_PUBLIC_API_URL || "http://localhost:9710").replace(/\/+$/, "");

// ApiError carries the server's error envelope: {"error": {code, message, request_id}}.
export class ApiError extends Error {
  constructor(status, code, message, requestId) {
    super(message);
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

async function send(path, { body, headers, token, ...options }) {
  const res = await fetch(`${API}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(headers || {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return { res, data: null };
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  return { res, data };
}

function toError(res, data) {
  const err = data && data.error;
  return new ApiError(
    res.status,
    err?.code || "http_error",
    err?.message || (typeof data === "string" && data) || `Request failed (${res.status})`,
    err?.request_id || res.headers.get("X-Request-Id") || undefined,
  );
}

// refreshSession exchanges the refresh cookie for a new access token. Calls
// are de-duplicated so parallel requests share one refresh.
let refreshing = null;
export function refreshSession() {
  if (!refreshing) {
    refreshing = (async () => {
      for (let attempt = 0; attempt < 2; attempt++) {
        const { res, data } = await send("/v1/auth/refresh", { method: "POST" });
        if (res.ok) {
          setSession(data);
          return data;
        }
        // Another tab rotated the cookie at the same moment: retry once.
        if (res.status === 409 && data?.error?.code === "refresh_race") {
          await new Promise((r) => setTimeout(r, 400));
          continue;
        }
        clearSession();
        return null;
      }
      return null;
    })().finally(() => { refreshing = null; });
  }
  return refreshing;
}

export async function apiFetch(path, { auth = true, ...options } = {}) {
  let token = auth ? getToken() : null;
  if (auth && !token && getSession()) token = (await refreshSession())?.token || null;
  let { res, data } = await send(path, { ...options, token });
  if (auth && res.status === 401) {
    const s = await refreshSession();
    if (s) ({ res, data } = await send(path, { ...options, token: s.token }));
    if (res.status === 401) {
      clearSession();
      if (typeof window !== "undefined") window.dispatchEvent(new Event("paytrack:unauthorized"));
    }
  }
  if (!res.ok) throw toError(res, data);
  return data;
}

// adminFetch calls the admin portal API. Staff sessions live in an httpOnly
// cookie scoped to /api/admin; writes must carry the X-Paytrack-Admin header.
async function adminFetch(path, options = {}) {
  const { res, data } = await send(path, { ...options, headers: { "X-Paytrack-Admin": "1", ...(options.headers || {}) } });
  if (res.status === 401 && typeof window !== "undefined") window.dispatchEvent(new Event("paytrack:admin-unauthorized"));
  if (!res.ok) throw toError(res, data);
  return data;
}

// getBaseUrl is the public API URL tenant backends call directly (docs, samples).
export function getBaseUrl() {
  return PUBLIC_API_URL;
}

// googleStartUrl is a full-page navigation target, not a fetch.
export function googleStartUrl(next = "/dashboard") {
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  return `${API}/v1/auth/google/start?next=${encodeURIComponent(next)}&origin=${encodeURIComponent(origin)}`;
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
const pub = { auth: false };

export const api = {
  admin: {
    // Returns {challenge: {id, masked_email, expires_at}}; the code is emailed.
    login: (email, password) => adminFetch("/admin/v1/auth/login", { method: "POST", body: { email, password } }),
    verify: (challenge_id, code) => adminFetch("/admin/v1/auth/verify", { method: "POST", body: { challenge_id, code } }),
    logout: () => adminFetch("/admin/v1/auth/logout", { method: "POST" }),
    me: () => adminFetch("/admin/v1/me"),
    auditLog: (params = {}) => adminFetch(`/admin/v1/audit-log${qs(params)}`),
    suspendTenant: (id, reason) => adminFetch(`/admin/v1/tenants/${enc(id)}/suspend`, { method: "POST", body: { reason } }),
    reinstateTenant: (id, reason) => adminFetch(`/admin/v1/tenants/${enc(id)}/reinstate`, { method: "POST", body: { reason } }),
    disableUser: (id, reason) => adminFetch(`/admin/v1/users/${enc(id)}/disable`, { method: "POST", body: { reason } }),
    enableUser: (id, reason) => adminFetch(`/admin/v1/users/${enc(id)}/enable`, { method: "POST", body: { reason } }),
    // kind: payments | webhooks | tenants | users | emails | support
    stats: (kind, days = 30) => adminFetch(`/admin/v1/stats/${kind}${qs({ days })}`),
    tenants: (params = {}) => adminFetch(`/admin/v1/tenants${qs(params)}`),
    tenant: (id) => adminFetch(`/admin/v1/tenants/${enc(id)}`),
    users: (params = {}) => adminFetch(`/admin/v1/users${qs(params)}`),
    user: (id) => adminFetch(`/admin/v1/users/${enc(id)}`),
    payments: (params = {}) => adminFetch(`/admin/v1/payments${qs(params)}`),
    deliveries: (params = {}) => adminFetch(`/admin/v1/webhook-deliveries${qs(params)}`),
    // action: send_password_reset | resend_verification | revoke_sessions | reset_two_step | unlink_google | clear_rate_limits
    recover: (id, action, reason) => adminFetch(`/admin/v1/users/${enc(id)}/recover`, { method: "POST", body: { action, reason } }),
    changeEmail: (id, email, reason) => adminFetch(`/admin/v1/users/${enc(id)}/change-email`, { method: "POST", body: { email, reason } }),
    support: {
      summary: () => adminFetch("/admin/v1/support/summary"),
      list: (params = {}) => adminFetch(`/admin/v1/support/tickets${qs(params)}`),
      get: (id) => adminFetch(`/admin/v1/support/tickets/${enc(id)}`),
      reply: (id, body, internal = false) => adminFetch(`/admin/v1/support/tickets/${enc(id)}/messages`, { method: "POST", body: { body, internal } }),
      // patch: {status?, priority?}
      update: (id, patch) => adminFetch(`/admin/v1/support/tickets/${enc(id)}`, { method: "PATCH", body: patch }),
      feedback: (params = {}) => adminFetch(`/admin/v1/support/feedback${qs(params)}`),
    },
  },

  inbox: {
    list: (params = {}) => apiFetch(`/v1/notifications${qs(params)}`),
    unreadCount: () => apiFetch("/v1/notifications/unread-count"),
    markRead: (id) => apiFetch(`/v1/notifications/${enc(id)}/read`, { method: "POST" }),
    markAllRead: () => apiFetch("/v1/notifications/read-all", { method: "POST" }),
  },

  support: {
    list: (params = {}) => apiFetch(`/v1/support/tickets${qs(params)}`),
    create: (body) => apiFetch("/v1/support/tickets", { method: "POST", body }),
    get: (id) => apiFetch(`/v1/support/tickets/${enc(id)}`),
    reply: (id, body) => apiFetch(`/v1/support/tickets/${enc(id)}/messages`, { method: "POST", body: { body } }),
    rate: (id, rating, comment) => apiFetch(`/v1/support/tickets/${enc(id)}/rating`, { method: "POST", body: { rating, comment } }),
    feedback: (body) => apiFetch("/v1/support/feedback", { method: "POST", body }),
  },

  auth: {
    providers: () => apiFetch("/v1/auth/providers", pub),
    register: (body) => apiFetch("/v1/auth/register", { method: "POST", body, ...pub }),
    // Returns a session, or {mfa_required: true, challenge: {id, masked_email, expires_at}}.
    login: (body) => apiFetch("/v1/auth/login", { method: "POST", body, ...pub }),
    verifyCode: (body) => apiFetch("/v1/auth/login/verify", { method: "POST", body, ...pub }),
    resendCode: (challenge_id) => apiFetch("/v1/auth/login/resend", { method: "POST", body: { challenge_id }, ...pub }),
    logout: () => send("/v1/auth/logout", { method: "POST", token: getToken() }),
    verifyEmail: (token) => apiFetch("/v1/auth/verify-email", { method: "POST", body: { token }, ...pub }),
    resendVerification: () => apiFetch("/v1/auth/verify-email/resend", { method: "POST" }),
    forgotPassword: (email) => apiFetch("/v1/auth/password/forgot", { method: "POST", body: { email }, ...pub }),
    resetPassword: (token, password) => apiFetch("/v1/auth/password/reset", { method: "POST", body: { token, password }, ...pub }),
    googleSignupInfo: (token) => apiFetch(`/v1/auth/google/signup${qs({ token })}`, pub),
    googleComplete: (token, business_name) => apiFetch("/v1/auth/google/complete", { method: "POST", body: { token, business_name }, ...pub }),
    me: () => apiFetch("/v1/me"),
  },

  account: {
    changePassword: (current_password, new_password) => apiFetch("/v1/me/password", { method: "POST", body: { current_password, new_password } }),
    setTwoStep: (enabled, password) => apiFetch("/v1/me/two-step", { method: "PUT", body: { enabled, password } }),
    sessions: () => apiFetch("/v1/me/sessions"),
    revokeSession: (id) => apiFetch(`/v1/me/sessions/${enc(id)}`, { method: "DELETE" }),
    revokeOtherSessions: () => apiFetch("/v1/me/sessions/revoke-others", { method: "POST" }),
    securityEvents: (limit = 20) => apiFetch(`/v1/me/security-events${qs({ limit })}`),
    linkGoogle: () => apiFetch("/v1/me/identities/google", { method: "POST", body: { origin: window.location.origin } }),
    unlinkGoogle: () => apiFetch("/v1/me/identities/google", { method: "DELETE" }),
  },

  onboarding: () => apiFetch("/v1/onboarding"),

  notifications: {
    get: () => apiFetch("/v1/settings/notifications"),
    update: (patch) => apiFetch("/v1/settings/notifications", { method: "PUT", body: patch }),
    preview: (template = "payment_receipt") => apiFetch(`/v1/settings/notifications/preview${qs({ template })}`),
    testReceipt: () => apiFetch("/v1/settings/notifications/test-receipt", { method: "POST" }),
  },

  apiKeys: {
    list: () => apiFetch("/v1/api-keys"),
    create: (name) => apiFetch("/v1/api-keys", { method: "POST", body: { name } }),
    revoke: (id) => apiFetch(`/v1/api-keys/${enc(id)}`, { method: "DELETE" }),
  },

  banks: {
    list: () => apiFetch("/v1/banks", pub),
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
    // checkStatus asks M-Pesa for the result now: {payment, outcome, message}.
    checkStatus: (id) => apiFetch(`/v1/payments/${enc(id)}/check-status`, { method: "POST" }),
    // idempotencyKey must be stable across retries of the same charge.
    create: (payment, idempotencyKey) =>
      apiFetch("/v1/payments", { method: "POST", body: payment, headers: { "Idempotency-Key": idempotencyKey } }),
  },
};
