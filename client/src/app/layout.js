import { Figtree } from "next/font/google";
import "./globals.css";
import { AuthProvider } from "../context/AuthContext.jsx";

const figtree = Figtree({
  variable: "--font-figtree",
  subsets: ["latin"],
  display: "swap",
});

export const metadata = {
  metadataBase: new URL("https://payments.geninworld.com"),
  title: {
    default: "Paytrack — M‑Pesa integration for developers",
    template: "%s — Paytrack",
  },
  description: "Build with M‑Pesa faster: STK Push, C2B/B2C/B2B, signed webhooks, idempotency, wallet and payouts.",
  openGraph: {
    title: "Paytrack — M‑Pesa integration for developers",
    description: "Build with M‑Pesa faster: STK Push, C2B/B2C/B2B, signed webhooks, idempotency, wallet and payouts.",
    siteName: "Paytrack",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Paytrack — M‑Pesa integration for developers",
    description: "Build with M‑Pesa faster: STK Push, C2B/B2C/B2B, signed webhooks, idempotency, wallet and payouts.",
  },
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body className={`${figtree.variable}`}>
        <AuthProvider>
          {children}
        </AuthProvider>
      </body>
    </html>
  );
}
