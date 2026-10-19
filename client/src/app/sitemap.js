import { SITE } from "@/lib/site";
import { LEGAL } from "@/lib/legal";

// Only public, indexable pages. Dashboard, admin, auth flows and receipts are
// private and excluded (see robots.js).
export default function sitemap() {
  const now = new Date();
  const legal = new Date(LEGAL.version);
  return [
    { url: `${SITE.url}/`, lastModified: now, changeFrequency: "weekly", priority: 1 },
    { url: `${SITE.url}/docs`, lastModified: now, changeFrequency: "weekly", priority: 0.9 },
    { url: `${SITE.url}/terms`, lastModified: legal, changeFrequency: "yearly", priority: 0.3 },
    { url: `${SITE.url}/privacy`, lastModified: legal, changeFrequency: "yearly", priority: 0.3 },
    { url: `${SITE.url}/disclaimer`, lastModified: legal, changeFrequency: "yearly", priority: 0.3 },
  ];
}
