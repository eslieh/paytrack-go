"use client";

import Link from "next/link";
import { Mail, ArrowLeft } from "lucide-react";
import { useState } from "react";
import { api } from "@/lib/api";
import { ErrorLine, IconField, SuccessLine } from "@/components/auth.jsx";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await api.auth.forgotPassword(email.trim());
      setDone(true);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Reset your password</h1>
        <p className="card-subtitle">Enter your account email and we'll send you a reset link.</p>
      </div>
      <div className="card-body">
        {done ? (
          <div className="stack">
            <SuccessLine>If an account exists for <strong>{email}</strong>, a reset link is on its way. It expires in 30 minutes.</SuccessLine>
            <Link href="/auth" className="btn btn-primary">Back to sign in</Link>
          </div>
        ) : (
          <form className="stack" onSubmit={onSubmit}>
            <IconField id="email" label="Email" icon={Mail}>
              <input id="email" type="email" autoComplete="email" className="input" placeholder="you@example.com" style={{ paddingLeft: 38 }} value={email} onChange={(e) => setEmail(e.target.value)} required />
            </IconField>
            <ErrorLine error={error} />
            <button className="btn btn-primary" type="submit" disabled={submitting}>{submitting ? "Sending..." : "Send reset link"}</button>
            <Link href="/auth" className="btn btn-ghost" style={{ padding: 0, justifyContent: "flex-start", fontSize: 13 }}>
              <ArrowLeft size={16} /> Back to sign in
            </Link>
          </form>
        )}
      </div>
    </div>
  );
}
