"use client";

import Link from "next/link";
import { Mail, Lock, ArrowRight } from "lucide-react";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/context/AuthContext.jsx";
import { AUTH_ERRORS, ErrorLine, GoogleButton, IconField, PasswordInput, safeNext } from "@/components/auth.jsx";
import { Busy } from "@/components/Busy.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <Login />
    </Suspense>
  );
}

function Login() {
  const params = useSearchParams();
  const next = safeNext(params.get("next"));
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(AUTH_ERRORS[params.get("error")] || "");
  const router = useRouter();
  const { login, token, loading } = useAuth();

  useEffect(() => {
    if (!loading && token) router.replace(next);
  }, [loading, token, next, router]);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const res = await login(email.trim(), password);
      if (res.challenge) {
        const q = new URLSearchParams({ c: res.challenge.id, e: res.challenge.masked_email, next });
        startTopLoader(); router.push(`/auth/verify-code?${q}`);
        return;
      }
      startTopLoader(); router.push(next);
    } catch (err) {
      setError(err.message || "Sign in failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Sign in to Paytrack</h1>
        <p className="card-subtitle">Manage your M-Pesa channels, payments and webhooks</p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={onSubmit}>
          <GoogleButton next={next} />

          <IconField id="email" label="Email" icon={Mail}>
            <input id="email" type="email" autoComplete="email" className="input" placeholder="you@example.com" style={{ paddingLeft: 38 }} value={email} onChange={(e) => setEmail(e.target.value)} required />
          </IconField>

          <label className="stack" htmlFor="password" style={{ gap: 6 }}>
            <span className="row">
              <span style={{ fontSize: 13, color: "#555b73", fontWeight: 600 }}>Password</span>
              <Link href="/auth/forgot" style={{ fontSize: 13, color: "#6472f3", fontWeight: 600 }}>Forgot password?</Link>
            </span>
            <div style={{ position: "relative", width: "100%" }}>
              <Lock size={18} color="#7b82a1" style={{ position: "absolute", left: 12, top: 12 }} />
              <PasswordInput id="password" value={password} onChange={setPassword} />
            </div>
          </label>

          <ErrorLine error={error} />

          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? <Busy>Signing in…</Busy> : "Sign in"}
            <ArrowRight size={18} />
          </button>

          <div className="row" style={{ marginTop: 8 }}>
            <span style={{ fontSize: 13, color: "#667085" }}>No account?</span>
            <Link href="/auth/register" className="btn btn-ghost" style={{ padding: 0 }}>Create one</Link>
          </div>
        </form>
        <p style={{ marginTop: 16, fontSize: 12, color: "#8a91ad", lineHeight: 1.5, textAlign: "center" }}>
          Continuing with Google for the first time creates an account, which means you agree to the{" "}
          <Link href="/terms" style={{ color: "#3846f7" }}>Terms</Link>,{" "}
          <Link href="/privacy" style={{ color: "#3846f7" }}>Privacy Policy</Link> and{" "}
          <Link href="/disclaimer" style={{ color: "#3846f7" }}>Disclaimer</Link>.
        </p>
      </div>
    </div>
  );
}
