"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { LogOut } from "lucide-react";
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
];

// DashboardShell guards every /dashboard page behind a session and renders the nav.
export default function DashboardShell({ children }) {
  const { token, loading, logout } = useAuth();
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
          <button
            className={tabStyles.tab}
            onClick={() => { logout(); router.replace("/auth"); }}
            title="Sign out"
            aria-label="Sign out"
          >
            <LogOut size={16} />
          </button>
        }
      />
      <main className={styles.main}>{loading || !token ? <div className={styles.skeletonLine} /> : children}</main>
    </div>
  );
}
