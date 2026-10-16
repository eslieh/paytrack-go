"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, getBaseUrl, refreshSession } from "../lib/api";
import { clearSession, getSession, onSessionChange, setSession } from "../lib/storage";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [session, setState] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const off = onSessionChange(setState);
    // Restore the session from the refresh cookie (no tokens in storage).
    refreshSession().finally(() => {
      setState(getSession());
      setLoading(false);
    });
    const onUnauthorized = () => setState(null);
    window.addEventListener("paytrack:unauthorized", onUnauthorized);
    return () => {
      off();
      window.removeEventListener("paytrack:unauthorized", onUnauthorized);
    };
  }, []);

  const start = useCallback((s) => {
    setSession(s);
    return s;
  }, []);

  // login resolves to {session} or {challenge} when an email code is needed.
  const login = useCallback(async (email, password) => {
    const res = await api.auth.login({ email, password });
    if (res.mfa_required) return { challenge: res.challenge };
    return { session: start(res) };
  }, [start]);

  const verifyCode = useCallback(
    async (challengeId, code, rememberDevice) =>
      start(await api.auth.verifyCode({ challenge_id: challengeId, code, remember_device: rememberDevice })),
    [start],
  );

  const signup = useCallback(
    async (businessName, email, password) =>
      start(await api.auth.register({ business_name: businessName, email, password })),
    [start],
  );

  // Revoke the session on the server, then forget it locally.
  const logout = useCallback(async () => {
    try { await api.auth.logout(); } catch {}
    clearSession();
  }, []);

  // reload re-reads the user (e.g. after verifying the email).
  const reload = useCallback(async () => {
    const me = await api.auth.me();
    const cur = getSession();
    if (cur && me.user) setSession({ ...cur, user: me.user, tenant: me.tenant });
    return me;
  }, []);

  const value = useMemo(
    () => ({
      session,
      user: session?.user || null,
      tenant: session?.tenant || null,
      token: session?.token || null,
      loading,
      login,
      verifyCode,
      signup,
      start,
      logout,
      reload,
      baseUrl: getBaseUrl(),
    }),
    [session, loading, login, verifyCode, signup, start, logout, reload],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  return useContext(AuthContext);
}
