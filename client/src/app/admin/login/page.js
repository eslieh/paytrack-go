"use client";

import { Mail, Lock, ArrowRight, ShieldCheck } from "lucide-react";
import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { ErrorLine, IconField, OtpInput, PasswordInput } from "@/components/auth.jsx";
import { Busy } from "@/components/Busy.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";

export default function AdminLoginPage() {
  return (
    <Suspense fallback={null}>
      <AdminLogin />
    </Suspense>
  );
}

// Admin sign-in always takes a password and an emailed code.
function AdminLogin() {
  const params = useSearchParams();
  const raw = params.get("next") || "";
  const next = raw.startsWith("/admin") && !raw.startsWith("//") ? raw : "/admin";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState(null);
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();

  async function signIn(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      setChallenge((await api.admin.login(email.trim(), password)).challenge);
    } catch (err) {
      setError(err.message || "Sign in failed");
    } finally {
      setSubmitting(false);
    }
  }

  async function verify(value = code) {
    if (value.length !== 6 || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await api.admin.verify(challenge.id, value);
      startTopLoader(); router.replace(next);
    } catch (err) {
      setError(err.message);
      setCode("");
      if (["too_many_attempts", "code_expired", "invalid_challenge", "code_used"].includes(err.code)) {
        setChallenge(null);
        setPassword("");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="centered-screen">
      <div className="card">
        {!challenge ? (
          <>
            <div className="card-header">
              <h1 className="card-title">Paytrack Admin</h1>
              <p className="card-subtitle">Staff sign-in</p>
            </div>
            <div className="card-body">
              <form className="stack" onSubmit={signIn}>
                <IconField id="email" label="Email" icon={Mail}>
                  <input id="email" type="email" autoComplete="username" className="input" style={{ paddingLeft: 38 }} value={email} onChange={(e) => setEmail(e.target.value)} required />
                </IconField>
                <label className="stack" htmlFor="password" style={{ gap: 6 }}>
                  <span style={{ fontSize: 13, color: "#555b73", fontWeight: 600 }}>Password</span>
                  <div style={{ position: "relative", width: "100%" }}>
                    <Lock size={18} color="#7b82a1" style={{ position: "absolute", left: 12, top: 12 }} />
                    <PasswordInput id="password" value={password} onChange={setPassword} />
                  </div>
                </label>
                <ErrorLine error={error} />
                <button className="btn btn-primary" type="submit" disabled={submitting}>
                  {submitting ? <Busy>Checking…</Busy> : "Continue"}
                  <ArrowRight size={18} />
                </button>
              </form>
            </div>
          </>
        ) : (
          <>
            <div className="card-header">
              <div style={{ width: 40, height: 40, borderRadius: 12, background: "#eef0ff", display: "grid", placeItems: "center", marginBottom: 12 }}>
                <ShieldCheck size={22} color="#6472f3" />
              </div>
              <h1 className="card-title">Check your email</h1>
              <p className="card-subtitle">Enter the 6-digit code we sent to <strong style={{ color: "#0f1222" }}>{challenge.masked_email}</strong>.</p>
            </div>
            <div className="card-body">
              <form className="stack" onSubmit={(e) => { e.preventDefault(); verify(); }}>
                <OtpInput value={code} onChange={setCode} onComplete={verify} disabled={submitting} />
                <ErrorLine error={error} />
                <button className="btn btn-primary" type="submit" disabled={submitting || code.length !== 6}>
                  {submitting ? <Busy>Verifying…</Busy> : "Verify and sign in"}
                </button>
                <button type="button" className="btn btn-ghost" style={{ padding: 0, fontSize: 13 }} onClick={() => { setChallenge(null); setCode(""); setError(""); }}>
                  Start over
                </button>
              </form>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
