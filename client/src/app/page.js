import Landing from "./landing.jsx";
import { SITE, jsonLd, pageMeta } from "@/lib/site";

export const metadata = pageMeta({ description: SITE.description, path: "/" });

// Structured data: the site, the product, and who built it.
const STRUCTURED = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "WebSite",
      "@id": `${SITE.url}/#website`,
      url: SITE.url,
      name: SITE.name,
      description: SITE.description,
      inLanguage: "en-KE",
      creator: { "@id": `${SITE.url}/#author` },
    },
    {
      "@type": "SoftwareApplication",
      "@id": `${SITE.url}/#app`,
      name: SITE.name,
      url: SITE.url,
      applicationCategory: "DeveloperApplication",
      applicationSubCategory: "Payments API",
      operatingSystem: "Web",
      description: SITE.description,
      featureList: [
        "M-Pesa STK Push via one REST call",
        "Routing to bank accounts, PayBills and Tills",
        "HMAC-SHA256 signed webhooks with ~16h of retries",
        "Idempotent payment creation",
        "Non-custodial: funds never held by Paytrack",
      ],
      areaServed: { "@type": "Country", name: "Kenya" },
      author: { "@id": `${SITE.url}/#author` },
    },
    {
      "@type": "Person",
      "@id": `${SITE.url}/#author`,
      name: SITE.author.name,
      url: SITE.author.url,
      sameAs: SITE.author.sameAs,
    },
  ],
};

export default function Page() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={jsonLd(STRUCTURED)} />
      <Landing />
    </>
  );
}
