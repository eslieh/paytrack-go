"use client";

import Link from "next/link";
import { Mail, Lock, Building2, ArrowRight } from "lucide-react";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorLine, GoogleButton, IconField, PasswordInput, StrengthMeter } from "@/components/auth.jsx";

export default function RegisterPage() {
  const [businessName, setBusinessName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();
  const { signup, token, loading } = useAuth();
  const [justSignedUp, setJustSignedUp] = useState(false);

  useEffect(() => {
    if (!loading && token && !justSignedUp) router.replace("/dashboard");
  }, [loading, token, justSignedUp, router]);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      setJustSignedUp(true);
      await signup(businessName.trim(), email.trim(), password);
      router.push("/auth/check-email");
    } catch (err) {
      setJustSignedUp(false);
      setError(err.message || "Sign up failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Create your Paytrack account</h1>
        <p className="card-subtitle">Collect M-Pesa payments into your bank account, PayBill or Till</p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={onSubmit}>
          <GoogleButton label="Sign up with Google" />

          <IconField id="business_name" label="Business name" icon={Building2}>
            <input id="business_name" className="input" placeholder="Acme Shop" maxLength={100} autoComplete="organization" style={{ paddingLeft: 38 }} value={businessName} onChange={(e) => setBusinessName(e.target.value)} required />
          </IconField>

          <IconField id="email" label="Email" icon={Mail} hint="We'll send a link to verify it.">
            <input id="email" type="email" autoComplete="email" className="input" placeholder="you@example.com" style={{ paddingLeft: 38 }} value={email} onChange={(e) => setEmail(e.target.value)} required />
          </IconField>

          <IconField id="password" label="Password" icon={Lock}>
            <PasswordInput id="password" value={password} onChange={setPassword} autoComplete="new-password" placeholder="At least 10 characters" minLength={10} />
          </IconField>
          <StrengthMeter password={password} />

          <ErrorLine error={error} />

          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? "Creating..." : "Create account"}
            <ArrowRight size={18} />
          </button>

          <div className="row" style={{ marginTop: 8 }}>
            <span style={{ fontSize: 13, color: "#667085" }}>Already have an account?</span>
            <Link href="/auth" className="btn btn-ghost" style={{ padding: 0 }}>Sign in</Link>
          </div>
        </form>
      </div>
    </div>
  );
}
