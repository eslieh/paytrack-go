// Dashboard session state. The access token lives only in memory; the
// long-lived refresh token is an httpOnly cookie the browser sends to
// /api/v1/auth/refresh, so a page reload restores the session without ever
// exposing a token to JavaScript storage.

let session = null; // {token, expires_at, user, tenant}
const listeners = new Set();

export function setSession(s) {
  session = s && s.token ? s : null;
  listeners.forEach((fn) => fn(session));
}

export function getSession() {
  return session;
}

export function getToken() {
  if (!session) return null;
  // Treat tokens about to expire as gone so the client refreshes first.
  if (session.expires_at && Date.parse(session.expires_at) - Date.now() < 15_000) return null;
  return session.token;
}

export function onSessionChange(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function clearSession() {
  setSession(null);
  // Remove storage used by older client versions.
  try {
    localStorage.removeItem("paytrack_session");
    localStorage.removeItem("paytrack_token");
    localStorage.removeItem("paytrack_user");
    document.cookie = "paytrack_token=; Max-Age=0; path=/";
    document.cookie = "paytrack_user=; Max-Age=0; path=/";
  } catch {}
}
