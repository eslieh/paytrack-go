import { Figtree } from "next/font/google";
import { SITE } from "../lib/site";
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
  metadataBase: new URL(SITE.url),
  title: {
    default: SITE.title,
    template: `%s | ${SITE.name}`,
  },
  description: SITE.description,
  applicationName: SITE.name,
  keywords: SITE.keywords,
  authors: [{ name: SITE.author.name, url: SITE.author.url }],
  creator: SITE.author.name,
  publisher: SITE.name,
  category: "technology",
  openGraph: {
    type: "website",
    url: "/",
    siteName: SITE.name,
    locale: SITE.locale,
    title: SITE.title,
    description: SITE.description,
  },
  twitter: {
    card: "summary_large_image",
    site: SITE.twitter,
    creator: SITE.twitter,
    title: SITE.title,
    description: SITE.description,
  },
  robots: {
    index: true,
    follow: true,
    googleBot: { index: true, follow: true, "max-image-preview": "large", "max-snippet": -1, "max-video-preview": -1 },
  },
  formatDetection: { telephone: false, email: false, address: false },
};

export const viewport = {
  themeColor: SITE.themeColor,
  colorScheme: "light",
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }) {
  return (
    <html lang="en-KE">
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
