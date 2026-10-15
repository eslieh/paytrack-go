import DashboardShell from "@/components/DashboardShell.jsx";

export const metadata = {
  title: {
    default: "Dashboard",
    template: "%s — Paytrack",
  },
  description: "Manage your Daraja channels, STK payments, webhooks and API keys.",
  robots: { index: false, follow: false },
};

export default function DashboardLayout({ children }) {
  return <DashboardShell>{children}</DashboardShell>;
}
