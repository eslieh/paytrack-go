"use client";

import Link from "next/link";
import { Mail, Lock, ArrowRight, Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const [showPassword, setShowPassword] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();

  const BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:5000";

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const res = await fetch(`${BASE_URL}/auth/dashboard/tenant/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Login failed (${res.status})`);
      }
      const data = await res.json();
      localStorage.setItem("paytrack_token", data.access_token);
      localStorage.setItem("paytrack_user", JSON.stringify(data.user));
      router.push("/dashboard");
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
        <p className="card-subtitle">Manage your finances, payments and invoices</p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={onSubmit}>
          <label className="stack" htmlFor="email">
            <span style={{fontSize: 13, color: "#555b73", fontWeight: 600}}>Email</span>
            <div className="row" style={{gap: 8}}>
              <div style={{position: "relative", width: "100%"}}>
                <Mail size={18} color="#7b82a1" style={{position: "absolute", left: 12, top: 12}} />
                <input id="email" type="email" className="input" placeholder="you@example.com" style={{paddingLeft: 38}} value={email} onChange={(e)=>setEmail(e.target.value)} required />
              </div>
            </div>
          </label>

          <label className="stack" htmlFor="password">
            <span style={{fontSize: 13, color: "#555b73", fontWeight: 600}}>Password</span>
            <div style={{position: "relative", width: "100%"}}>
              <Lock size={18} color="#7b82a1" style={{position: "absolute", left: 12, top: 12}} />
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                className="input"
                placeholder="••••••••"
                style={{paddingLeft: 38, paddingRight: 38}}
                value={password}
                onChange={(e)=>setPassword(e.target.value)}
                required
              />
              {showPassword ? (
                <EyeOff
                  size={18}
                  color="#7b82a1"
                  style={{position: "absolute", right: 12, top: 12, cursor: "pointer"}}
                  onClick={() => setShowPassword(false)}
                  aria-label="Hide password"
                />
              ) : (
                <Eye
                  size={18}
                  color="#7b82a1"
                  style={{position: "absolute", right: 12, top: 12, cursor: "pointer"}}
                  onClick={() => setShowPassword(true)}
                  aria-label="Show password"
                />
              )}
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


