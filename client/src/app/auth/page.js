"use client";

import Link from "next/link";
import { Mail, Lock, ArrowRight, Eye, EyeOff } from "lucide-react";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/context/AuthContext.jsx";

// Only follow same-site relative redirects.
function safeNext(next) {
  return next && next.startsWith("/") && !next.startsWith("//") ? next : "/dashboard";
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <Login />
    </Suspense>
  );
}

function Login() {
  const [showPassword, setShowPassword] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();
  const next = safeNext(useSearchParams().get("next"));
  const { login, token, loading } = useAuth();

  useEffect(() => {
    if (!loading && token) router.replace(next);
  }, [loading, token, next, router]);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await login(email.trim(), password);
      router.push(next);
    } catch (err) {
      setError(err.message || "Login failed");
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
          <label className="stack" htmlFor="email">
            <span style={{fontSize: 13, color: "#555b73", fontWeight: 600}}>Email</span>
            <div style={{position: "relative", width: "100%"}}>
              <Mail size={18} color="#7b82a1" style={{position: "absolute", left: 12, top: 12}} />
              <input id="email" type="email" autoComplete="email" className="input" placeholder="you@example.com" style={{paddingLeft: 38}} value={email} onChange={(e)=>setEmail(e.target.value)} required />
            </div>
          </label>

          <label className="stack" htmlFor="password">
            <span style={{fontSize: 13, color: "#555b73", fontWeight: 600}}>Password</span>
            <div style={{position: "relative", width: "100%"}}>
              <Lock size={18} color="#7b82a1" style={{position: "absolute", left: 12, top: 12}} />
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                className="input"
                placeholder="••••••••"
                style={{paddingLeft: 38, paddingRight: 38}}
                value={password}
                onChange={(e)=>setPassword(e.target.value)}
                required
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? "Hide password" : "Show password"}
                style={{position: "absolute", right: 12, top: 12, background: "none", border: 0, padding: 0, cursor: "pointer"}}
              >
                {showPassword ? <EyeOff size={18} color="#7b82a1" /> : <Eye size={18} color="#7b82a1" />}
              </button>
            </div>
          </label>

          {error && (<div style={{color: "#b42318", fontSize: 13}}>{error}</div>)}

          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? "Signing in..." : "Sign in"}
            <ArrowRight size={18} />
          </button>

          <div className="row" style={{marginTop: 8}}>
            <span style={{fontSize: 13, color: "#667085"}}>No account?</span>
            <Link href="/auth/register" className="btn btn-ghost" style={{padding: 0}}>Create one</Link>
          </div>
        </form>
      </div>
    </div>
  );
}
