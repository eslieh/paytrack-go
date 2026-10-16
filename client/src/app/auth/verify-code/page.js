"use client";

import Link from "next/link";
import { ShieldCheck } from "lucide-react";
import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorLine, OtpInput, safeNext, useCountdown } from "@/components/auth.jsx";
import { Busy } from "@/components/Busy.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";

export default function VerifyCodePage() {
  return (
    <Suspense fallback={null}>
      <VerifyCode />
    </Suspense>
  );
}

function VerifyCode() {
  const params = useSearchParams();
  const challengeId = params.get("c");
  const masked = params.get("e") || "your email";
  const next = safeNext(params.get("next"));
  const [code, setCode] = useState("");
  const [remember, setRemember] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [left, restart] = useCountdown(30);
  const router = useRouter();
  const { verifyCode } = useAuth();

  async function submit(value = code) {
    if (value.length !== 6 || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await verifyCode(challengeId, value, remember);
      startTopLoader(); router.push(next);
    } catch (err) {
      setError(err.message);
      setCode("");
      if (["too_many_attempts", "token_expired", "invalid_challenge", "token_used"].includes(err.code)) {
        setTimeout(() => router.replace(`/auth?next=${encodeURIComponent(next)}`), 2500);
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function resend() {
    setError("");
    setNotice("");
    try {
      await api.auth.resendCode(challengeId);
      setNotice(`We sent a new code to ${masked}.`);
      restart();
    } catch (err) {
      setError(err.message);
    }
  }

  if (!challengeId) {
    return (
      <div className="card-body" style={{ paddingTop: 24 }}>
        <p>This sign-in link is incomplete. <Link href="/auth" style={{ color: "#6472f3" }}>Sign in again</Link>.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="card-header">
        <div style={{ width: 40, height: 40, borderRadius: 12, background: "#eef0ff", display: "grid", placeItems: "center", marginBottom: 12 }}>
          <ShieldCheck size={22} color="#6472f3" />
        </div>
        <h1 className="card-title">Check your email</h1>
        <p className="card-subtitle">Enter the 6-digit code we sent to <strong style={{ color: "#0f1222" }}>{masked}</strong>.</p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={(e) => { e.preventDefault(); submit(); }}>
          <OtpInput value={code} onChange={setCode} onComplete={submit} disabled={submitting} />

          <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 13, color: "#344054", cursor: "pointer" }}>
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
            Remember this device for 30 days
          </label>

          <ErrorLine error={error} />
          {notice && <div style={{ fontSize: 13, color: "#067647" }}>{notice}</div>}

          <button className="btn btn-primary" type="submit" disabled={submitting || code.length !== 6}>
            {submitting ? <Busy>Verifying…</Busy> : "Verify and sign in"}
          </button>

          <div className="row" style={{ marginTop: 4, fontSize: 13 }}>
            <button type="button" className="btn btn-ghost" style={{ padding: 0, fontSize: 13, color: left ? "#8a91ad" : "#6472f3" }} disabled={left > 0} onClick={resend}>
              {left > 0 ? `Resend code in ${left}s` : "Resend code"}
            </button>
            <Link href="/auth" style={{ color: "#667085" }}>Use a different account</Link>
          </div>
        </form>
      </div>
    </div>
  );
}
