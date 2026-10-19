import { SITE, jsonLd, pageMeta } from "@/lib/site";

export const metadata = pageMeta({
  title: "API Reference",
  description:
    "Paytrack API reference: create M-Pesa STK Push payments, route them to bank accounts, PayBills or Tills, verify HMAC-signed webhooks, handle errors, idempotency and pagination. Request and response examples in cURL, Node.js, Python and PHP.",
  path: "/docs",
});

const STRUCTURED = {
  "@context": "https://schema.org",
  "@type": "TechArticle",
  headline: "Paytrack API Reference",
  description: "How to collect M-Pesa payments with the Paytrack REST API.",
  url: `${SITE.url}/docs`,
  inLanguage: "en-KE",
  proficiencyLevel: "Beginner",
  about: ["M-Pesa", "STK Push", "Webhooks", "REST API"],
  author: { "@type": "Person", name: SITE.author.name, url: SITE.author.url, sameAs: SITE.author.sameAs },
  isPartOf: { "@type": "WebSite", "@id": `${SITE.url}/#website`, name: SITE.name, url: SITE.url },
};

export default function DocsLayout({ children }) {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={jsonLd(STRUCTURED)} />
      {children}
    </>
  );
}
