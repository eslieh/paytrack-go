"use client";

import Link from "next/link";
import { Mail, Lock, Building2, ArrowRight } from "lucide-react";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext.jsx";

export default function RegisterPage() {
  const [businessName, setBusinessName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();
  const { signup, token, loading } = useAuth();

  useEffect(() => {
    if (!loading && token) router.replace("/dashboard");
  }, [loading, token, router]);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await signup(businessName.trim(), email.trim(), password);
      router.push("/dashboard");
    } catch (err) {
      setError(err.message || "Signup failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Create your Paytrack account</h1>
        <p className="card-subtitle">Start collecting M-Pesa payments through your own PayBill or Till</p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={onSubmit}>
          <label className="stack" htmlFor="business_name">
            <span style={{fontSize: 13, color: "#555b73", fontWeight: 600}}>Business name</span>
            <div style={{position: "relative", width: "100%"}}>
              <Building2 size={18} color="#7b82a1" style={{position: "absolute", left: 12, top: 12}} />
              <input id="business_name" className="input" placeholder="Acme Shop" maxLength={100} autoComplete="organization" style={{paddingLeft: 38}} value={businessName} onChange={(e)=>setBusinessName(e.target.value)} required />
            </div>
          </label>

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
              <input id="password" type="password" autoComplete="new-password" minLength={10} maxLength={72} className="input" placeholder="At least 10 characters" style={{paddingLeft: 38}} value={password} onChange={(e)=>setPassword(e.target.value)} required />
            </div>
          </label>

          {error && (<div style={{color: "#b42318", fontSize: 13}}>{error}</div>)}

          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? "Creating..." : "Create account"}
            <ArrowRight size={18} />
          </button>

          <div className="row" style={{marginTop: 8}}>
            <span style={{fontSize: 13, color: "#667085"}}>Already have an account?</span>
            <Link href="/auth" className="btn btn-ghost" style={{padding: 0}}>Sign in</Link>
          </div>
        </form>
      </div>
    </div>
  );
}
