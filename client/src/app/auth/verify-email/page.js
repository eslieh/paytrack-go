"use client";

import Link from "next/link";
import { CheckCircle2, XCircle } from "lucide-react";
import { Suspense, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/context/AuthContext.jsx";

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={null}>
      <VerifyEmail />
    </Suspense>
  );
}

function VerifyEmail() {
  const token = useSearchParams().get("token");
  const { token: session, loading, reload } = useAuth();
  const [state, setState] = useState({ status: "pending" });
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current || loading) return;
    ran.current = true;
    if (!token) {
      setState({ status: "error", message: "This verification link is incomplete." });
      return;
    }
    api.auth.verifyEmail(token)
      .then(async () => {
        if (session) await reload().catch(() => {});
        setState({ status: "ok" });
      })
      .catch((err) => setState({ status: "error", message: err.message, code: err.code }));
  }, [token, session, loading, reload]);

  if (state.status === "pending") {
    return <div className="card-body" style={{ paddingTop: 24 }}>Verifying your email…</div>;
  }
  const ok = state.status === "ok";
  return (
    <div>
      <div className="card-header">
        {ok ? <CheckCircle2 size={36} color="#16a34a" /> : <XCircle size={36} color="#b42318" />}
        <h1 className="card-title" style={{ marginTop: 10 }}>{ok ? "Email verified" : "We couldn't verify your email"}</h1>
        <p className="card-subtitle">
          {ok ? "You're all set. You can now create API keys, channels and payments." : state.message}
        </p>
      </div>
      <div className="card-body stack">
        <Link href={session ? (ok ? "/dashboard" : "/auth/check-email") : "/auth"} className="btn btn-primary">
          {session ? (ok ? "Go to dashboard" : "Send a new link") : "Sign in"}
        </Link>
      </div>
    </div>
  );
}
