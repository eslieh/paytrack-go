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
  description: "M‑Pesa STK Push on your own PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
  openGraph: {
    title: "Paytrack — M‑Pesa integration for developers",
    description: "M‑Pesa STK Push on your own PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
    siteName: "Paytrack",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Paytrack — M‑Pesa integration for developers",
    description: "M‑Pesa STK Push on your own PayBill or Till, with signed webhooks, idempotent payments and a dashboard to track them.",
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
