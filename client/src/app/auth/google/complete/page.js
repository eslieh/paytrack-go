"use client";

import Link from "next/link";
import { Building2, ArrowRight } from "lucide-react";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorLine, IconField } from "@/components/auth.jsx";
import { Busy } from "@/components/Busy.jsx";
import { Skeleton } from "@/components/ui.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";

export default function GoogleCompletePage() {
  return (
    <Suspense fallback={null}>
      <Complete />
    </Suspense>
  );
}

function Complete() {
  const token = useSearchParams().get("token");
  const [info, setInfo] = useState(null);
  const [businessName, setBusinessName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const router = useRouter();
  const { start } = useAuth();

  useEffect(() => {
    if (!token) return;
    api.auth.googleSignupInfo(token).then(setInfo).catch((err) => setError(err.message));
  }, [token]);

  async function onSubmit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      start(await api.auth.googleComplete(token, businessName.trim()));
      startTopLoader(); router.push("/dashboard");
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  if (!token || (error && !info)) {
    return (
      <div className="card-body stack" style={{ paddingTop: 24 }}>
        <ErrorLine error={error || "This sign-up link is incomplete."} />
        <Link href="/auth/register" className="btn btn-primary">Start again</Link>
      </div>
    );
  }

  return (
    <div>
      <div className="card-header">
        <h1 className="card-title">Name your business</h1>
        <p className="card-subtitle">
          {info ? <>Signing up as <strong style={{ color: "#0f1222" }}>{info.email}</strong> with Google.</> : <Skeleton w={220} h={13} />}
        </p>
      </div>
      <div className="card-body">
        <form className="stack" onSubmit={onSubmit}>
          <IconField id="business_name" label="Business name" icon={Building2} hint="Shown on payment receipts and in the dashboard.">
            <input id="business_name" className="input" placeholder="Acme Shop" maxLength={100} autoComplete="organization" style={{ paddingLeft: 38 }} value={businessName} onChange={(e) => setBusinessName(e.target.value)} required autoFocus />
          </IconField>
          <ErrorLine error={error} />
          <button className="btn btn-primary" type="submit" disabled={submitting || !info}>
            {submitting ? <Busy>Creating…</Busy> : "Create account"}
            <ArrowRight size={18} />
          </button>
        </form>
      </div>
    </div>
  );
}
