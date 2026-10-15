"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, getBaseUrl } from "../lib/api";
import { clearSession, getSession, saveSession } from "../lib/storage";

const AuthContext = createContext({
  session: null,
  user: null,
  tenant: null,
  token: null,
  loading: true,
  login: async (_email, _password) => {},
  signup: async (_businessName, _email, _password) => {},
  logout: () => {},
  baseUrl: "",
});

export function AuthProvider({ children }) {
  const [session, setSession] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setSession(getSession());
    setLoading(false);
    const onUnauthorized = () => setSession(null);
    window.addEventListener("paytrack:unauthorized", onUnauthorized);
    return () => window.removeEventListener("paytrack:unauthorized", onUnauthorized);
  }, []);

  const start = useCallback((s) => {
    saveSession(s);
    setSession(s);
    return s;
  }, []);

  const login = useCallback(async (email, password) => start(await api.auth.login({ email, password })), [start]);

  const signup = useCallback(
    async (businessName, email, password) =>
      start(await api.auth.register({ business_name: businessName, email, password })),
    [start],
  );

  const logout = useCallback(() => {
    clearSession();
    setSession(null);
  }, []);

  const value = useMemo(
    () => ({
      session,
      user: session?.user || null,
      tenant: session?.tenant || null,
      token: session?.token || null,
      loading,
      login,
      signup,
      logout,
      baseUrl: getBaseUrl(),
    }),
    [session, loading, login, signup, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  return useContext(AuthContext);
}
