// Dashboard session persistence. The session is the response of
// /v1/auth/login or /v1/auth/register: {token, expires_at, user, tenant}.

export const SESSION_KEY = "paytrack_session";

export function saveSession(session) {
  try { localStorage.setItem(SESSION_KEY, JSON.stringify(session)); } catch {}
}

export function getSession() {
  let s = null;
  try { s = JSON.parse(localStorage.getItem(SESSION_KEY) || "null"); } catch { return null; }
  if (!s?.token) return null;
  if (s.expires_at && Date.parse(s.expires_at) <= Date.now()) {
    clearSession();
    return null;
  }
  return s;
}

export function getToken() {
  return getSession()?.token || null;
}

export function clearSession() {
  try { localStorage.removeItem(SESSION_KEY); } catch {}
  // Remove storage used by older client versions.
  try {
    localStorage.removeItem("paytrack_token");
    localStorage.removeItem("paytrack_user");
    document.cookie = "paytrack_token=; Max-Age=0; path=/";
    document.cookie = "paytrack_user=; Max-Age=0; path=/";
  } catch {}
}
