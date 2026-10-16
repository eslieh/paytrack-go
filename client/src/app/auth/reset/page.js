"use client";

import Link from "next/link";
import { Lock } from "lucide-react";
import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorLine, IconField, PasswordInput, StrengthMeter } from "@/components/auth.jsx";
import { Busy } from "@/components/Busy.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={null}>
      <Reset />
    </Suspense>
  );
}

function Reset() {
  const token = useSearchParams().get("token");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const router = useRouter();
  const { start } = useAuth();

  async function onSubmit(e) {
    e.preventDefault();
    if (password !== confirm) {
      setError({ message: "Passwords don't match." });
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      start(await api.auth.resetPassword(token, password));
      startTopLoader(); router.push("/dashboard");
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  const expired = error && ["token_expired", "token_used", "invalid_token"].includes(error.code);
  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Choose a new password</h1>
        <p className="card-subtitle">This signs you out of every other device.</p>
      </div>
      <div className="card-body">
        {!token || expired ? (
          <div className="stack">
            <ErrorLine error={error || "This reset link is incomplete."} />
            <Link href="/auth/forgot" className="btn btn-primary">Send a new link</Link>
          </div>
        ) : (
          <form className="stack" onSubmit={onSubmit}>
            <IconField id="password" label="New password" icon={Lock}>
              <PasswordInput id="password" value={password} onChange={setPassword} autoComplete="new-password" placeholder="At least 10 characters" minLength={10} />
            </IconField>
            <StrengthMeter password={password} />
            <IconField id="confirm" label="Confirm password" icon={Lock}>
              <PasswordInput id="confirm" value={confirm} onChange={setConfirm} autoComplete="new-password" minLength={10} />
            </IconField>
            <ErrorLine error={error} />
            <button className="btn btn-primary" type="submit" disabled={submitting}>{submitting ? <Busy>Saving…</Busy> : "Set password and sign in"}</button>
          </form>
        )}
      </div>
    </div>
  );
}
