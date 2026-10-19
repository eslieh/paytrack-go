"use client";

import { useEffect, useState } from "react";
import { PageSkeleton } from "./ui.jsx";
import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import { Ban, CreditCard, KeyRound, LayoutDashboard, LifeBuoy, MailWarning, Settings, Waypoints, Webhook } from "lucide-react";
import { api } from "../lib/api";
import Sidebar from "./Sidebar.jsx";
import { useAuth } from "../context/AuthContext.jsx";
import styles from "../app/dashboard/styles.module.css";
import { Busy } from "@/components/Busy.jsx";
import { startTopLoader } from "@/components/TopLoader.jsx";
import NotificationBell, { useUnread } from "@/components/NotificationBell.jsx";

const NAV = [
  { label: "Overview", href: "/dashboard", icon: LayoutDashboard },
  { label: "Payments", href: "/dashboard/payments", icon: CreditCard },
  { label: "Channels", href: "/dashboard/channels", icon: Waypoints },
  { label: "Webhooks", href: "/dashboard/webhooks", icon: Webhook },
  { label: "API Keys", href: "/dashboard/api-keys", icon: KeyRound },
  { label: "Support", href: "/dashboard/support", icon: LifeBuoy },
  { label: "Settings", href: "/dashboard/settings", icon: Settings },
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
        {state === "sent" ? "Link sent" : state === "sending" ? <Busy>Sending…</Busy> : "Resend link"}
      </button>
      {!["idle", "sending", "sent"].includes(state) && <span>{state}</span>}
    </div>
  );
}

// SuspendedBanner explains why changes fail while the tenant is suspended.
function SuspendedBanner() {
  return (
    <div className={styles.notice} style={{ display: "flex", alignItems: "center", gap: 10, background: "#fef3f2", borderColor: "#fecdca", color: "#912018" }}>
      <Ban size={18} />
      <span>This account is suspended. Your dashboard is read-only and your API keys are paused; payments already in progress will still complete. Check your email for details, or <Link href="/dashboard/support" style={{ color: "inherit", fontWeight: 600 }}>contact support</Link>.</span>
    </div>
  );
}

// DashboardShell guards every /dashboard page behind a session and renders the nav.
export default function DashboardShell({ children }) {
  const { token, loading, logout, user, tenant } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [unread, refreshUnread] = useUnread(!!token);

  useEffect(() => {
    if (!loading && !token) router.replace(`/auth?next=${encodeURIComponent(pathname)}`);
  }, [loading, token, pathname, router]);

  const active = NAV.filter((n) => pathname === n.href || pathname.startsWith(`${n.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0]?.href;

  return (
    <div className={styles.shell}>
      <Sidebar
        brand="Paytrack"
        items={NAV}
        activeHref={active}
        account={user ? { name: tenant?.name, email: user.email } : null}
        onSignOut={async () => { startTopLoader(); await logout(); router.replace("/auth"); }}
        actions={token ? <NotificationBell unread={unread} onChanged={refreshUnread} /> : null}
      />
      <main className={styles.main}>
        {loading || !token ? <PageSkeleton /> : (
          <>
            {tenant?.suspended && <SuspendedBanner />}
            {user && !user.email_verified && !tenant?.suspended && <VerifyBanner email={user.email} />}
            {children}
          </>
        )}
      </main>
    </div>
  );
}
