"use client";

import Link from "next/link";
import { MailCheck } from "lucide-react";
import { useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorLine, useCountdown } from "@/components/auth.jsx";

export default function CheckEmailPage() {
  const { user } = useAuth();
  const [left, restart] = useCountdown(60);
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);

  async function resend() {
    setError("");
    try {
      await api.auth.resendVerification();
      setSent(true);
      restart();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div>
      <div className="card-header">
        <div style={{ width: 40, height: 40, borderRadius: 12, background: "#eef0ff", display: "grid", placeItems: "center", marginBottom: 12 }}>
          <MailCheck size={22} color="#6472f3" />
        </div>
        <h1 className="card-title">Verify your email</h1>
        <p className="card-subtitle">
          We sent a verification link to <strong style={{ color: "#0f1222" }}>{user?.email || "your inbox"}</strong>.
          Open it to create API keys, channels and payments.
        </p>
      </div>
      <div className="card-body stack">
        <Link href="/dashboard" className="btn btn-primary">Continue to dashboard</Link>
        <button className="btn" style={{ border: "1px solid #e5e7ef" }} disabled={left > 0 || !user} onClick={resend}>
          {left > 0 ? `Resend link in ${left}s` : "Resend link"}
        </button>
        {sent && <div style={{ fontSize: 13, color: "#067647" }}>A new link is on its way.</div>}
        <ErrorLine error={error} />
        <p style={{ fontSize: 12, color: "#8a91ad" }}>Can't find it? Check spam, or make sure {user?.email || "the address"} is correct.</p>
      </div>
    </div>
  );
}
