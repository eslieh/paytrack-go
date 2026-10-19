"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Building2, History, Inbox, LayoutDashboard, MessageSquareHeart } from "lucide-react";
import { api } from "../lib/api";
import Sidebar from "./Sidebar.jsx";
import { PageSkeleton } from "./ui.jsx";
import styles from "../app/dashboard/styles.module.css";
import { startTopLoader } from "@/components/TopLoader.jsx";

const NAV = [
  { label: "Overview", href: "/admin", icon: LayoutDashboard },
  { label: "Accounts", href: "/admin/tenants", icon: Building2 },
  { label: "Support", href: "/admin/support", icon: Inbox },
  { label: "Feedback", href: "/admin/feedback", icon: MessageSquareHeart },
  { label: "Activity", href: "/admin/activity", icon: History },
];

const StaffContext = createContext(null);
export const useStaff = () => useContext(StaffContext);

// AdminShell guards every admin page behind a staff session (an httpOnly
// cookie, so the only way to know is to ask the API).
export default function AdminShell({ children }) {
  const [staff, setStaff] = useState(null);
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    const toLogin = () => router.replace(`/admin/login?next=${encodeURIComponent(pathname)}`);
    api.admin.me().then((r) => setStaff(r.staff)).catch(toLogin);
    window.addEventListener("paytrack:admin-unauthorized", toLogin);
    return () => window.removeEventListener("paytrack:admin-unauthorized", toLogin);
  }, [pathname, router]);

  const active = NAV.filter((n) => pathname === n.href || pathname.startsWith(`${n.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0]?.href;

  return (
    <StaffContext.Provider value={staff}>
      <div className={styles.shell}>
        <Sidebar
          brand="Paytrack Admin"
          brandHref="/admin"
          items={NAV}
          activeHref={active}
          account={staff ? { name: staff.name || "Admin", email: staff.email } : null}
          onSignOut={async () => {
            startTopLoader();
            try { await api.admin.logout(); } catch {}
            router.replace("/admin/login");
          }}
        />
        <main className={styles.main}>{staff ? children : <PageSkeleton />}</main>
      </div>
    </StaffContext.Provider>
  );
}
