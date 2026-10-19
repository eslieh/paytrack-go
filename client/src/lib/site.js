// One source of truth for SEO: names, URLs and copy used by metadata, the
// sitemap, robots.txt, the manifest, social images and structured data.
import { BUILDER } from "./legal";

const url = (process.env.NEXT_PUBLIC_SITE_URL || "https://paytrack.pesagrid.co.ke").replace(/\/+$/, "");

export const SITE = {
  url,
  name: "Paytrack",
  title: "Paytrack: M-Pesa STK Push API for developers",
  tagline: "Instant payment collection & routing, built for developers.",
  description:
    "Collect M-Pesa payments straight into your bank account, PayBill or Till with one API call. STK Push, HMAC-signed webhooks with automatic retries, idempotent payments and a dashboard. No Daraja app needed.",
  keywords: [
    "M-Pesa API",
    "M-Pesa STK Push",
    "STK Push API",
    "Daraja API alternative",
    "M-Pesa payments Kenya",
    "Lipa na M-Pesa API",
    "PayBill API",
    "Till number API",
    "payment API Kenya",
    "M-Pesa webhooks",
    "mobile money API",
    "developer payments API",
  ],
  locale: "en_KE",
  themeColor: "#6472f3",
  twitter: "@_eslieh",
  author: { name: BUILDER.name, url: BUILDER.links[0].href, sameAs: BUILDER.links.map((l) => l.href) },
};

// pageMeta builds per-page metadata with a canonical URL and matching
// Open Graph / Twitter tags, so shared links always show the right page.
export function pageMeta({ title, description, path = "/", images }) {
  const fullTitle = title ? `${title} | ${SITE.name}` : SITE.title;
  // A page that sets openGraph doesn't inherit the root share image, so default to it.
  // Segments with their own opengraph-image file (/, /docs) override this automatically.
  const ogImages = images || [{ url: "/opengraph-image", width: 1200, height: 630, alt: SITE.title }];
  const twImages = images || ["/twitter-image"];
  return {
    title: title || { absolute: SITE.title },
    description,
    alternates: { canonical: path },
    openGraph: {
      type: "website",
      url: path,
      siteName: SITE.name,
      locale: SITE.locale,
      title: fullTitle,
      description,
      images: ogImages,
    },
    twitter: {
      card: "summary_large_image",
      site: SITE.twitter,
      creator: SITE.twitter,
      title: fullTitle,
      description,
      images: twImages,
    },
  };
}

// JsonLd renders a structured-data block. Data is ours, but escape "<" anyway
// so no string can ever close the script tag.
export function jsonLd(data) {
  return { __html: JSON.stringify(data).replace(/</g, "\\u003c") };
}
