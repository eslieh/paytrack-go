import { Figtree } from "next/font/google";
import "./globals.css";
import "./motion.css";
import { AuthProvider } from "../context/AuthContext.jsx";
import { Suspense } from "react";
import Ripples from "../components/Ripples.jsx";
import TopLoader from "../components/TopLoader.jsx";

const figtree = Figtree({
  variable: "--font-figtree",
  subsets: ["latin"],
  display: "swap",
});

export const metadata = {
  metadataBase: new URL("https://paytrack.pesagrid.co.ke"),
  title: {
    default: "Paytrack — M‑Pesa integration for developers",
    template: "%s — Paytrack",
  },
  description: "M‑Pesa STK Push into your bank account, PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
  openGraph: {
    title: "Paytrack — M‑Pesa integration for developers",
    description: "M‑Pesa STK Push into your bank account, PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
    siteName: "Paytrack",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Paytrack — M‑Pesa integration for developers",
    description: "M‑Pesa STK Push into your bank account, PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
  },
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body className={`${figtree.variable}`}>
        {/* useSearchParams in TopLoader needs a Suspense boundary */}
        <Suspense fallback={null}><TopLoader /></Suspense>
        <AuthProvider>
          {children}
        </AuthProvider>
        <Ripples />
      </body>
    </html>
  );
}
