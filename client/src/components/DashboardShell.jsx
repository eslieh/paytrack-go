"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { LogOut, MailWarning } from "lucide-react";
import { api } from "../lib/api";
import Tabs from "./Tabs.jsx";
import tabStyles from "./tabs.module.css";
import { useAuth } from "../context/AuthContext.jsx";
import styles from "../app/dashboard/styles.module.css";

const NAV = [
  { label: "Overview", href: "/dashboard" },
  { label: "Payments", href: "/dashboard/payments" },
  { label: "Channels", href: "/dashboard/channels" },
  { label: "Webhooks", href: "/dashboard/webhooks" },
  { label: "API Keys", href: "/dashboard/api-keys" },
  { label: "Settings", href: "/dashboard/settings" },
];

// VerifyBanner nudges unverified owners; transacting endpoints reject them.
function VerifyBanner({ email }) {
  const [state, setState] = useState("idle");
  async function resend() {
    setState("sending");
    try { await api.auth.resendVerification(); setState("sent"); } catch (e) { setState(e.message); }
  }
  return (
    <div className={styles.notice} style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", background: "#fffaeb", borderColor: "#fedf89", color: "#93370d" }}>
      <MailWarning size={18} />
      <span style={{ flex: "1 1 260px" }}>Verify <strong>{email}</strong> to create API keys, channels and payments.</span>
      <button className="btn" style={{ padding: "6px 10px", border: "1px solid #fedf89", background: "#fff" }} disabled={state === "sending" || state === "sent"} onClick={resend}>
        {state === "sent" ? "Link sent" : state === "sending" ? "Sending..." : "Resend link"}
      </button>
      {!["idle", "sending", "sent"].includes(state) && <span>{state}</span>}
    </div>
  );
}

// DashboardShell guards every /dashboard page behind a session and renders the nav.
export default function DashboardShell({ children }) {
  const { token, loading, logout, user } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !token) router.replace(`/auth?next=${encodeURIComponent(pathname)}`);
  }, [loading, token, pathname, router]);

  const active = NAV.filter((n) => pathname === n.href || pathname.startsWith(`${n.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0]?.href;

  return (
    <div className={styles.shell}>
      <Tabs
        brand="Paytrack"
        items={NAV}
        activeHref={active}
        actions={
          <>
          <Link href="/docs" className={tabStyles.tab} target="_blank">Docs</Link>
          <button
            className={tabStyles.tab}
            onClick={async () => { await logout(); router.replace("/auth"); }}
            title="Sign out"
            aria-label="Sign out"
          >
            <LogOut size={16} />
          </button>
          </>
        }
      />
      <main className={styles.main}>
        {loading || !token ? <div className={styles.skeletonLine} /> : (
          <>
            {user && !user.email_verified && <VerifyBanner email={user.email} />}
            {children}
          </>
        )}
      </main>
    </div>
  );
}
