import { ogImage, OG_SIZE } from "@/lib/og";
import { SITE } from "@/lib/site";

export const alt = SITE.title;
export const size = OG_SIZE;
export const contentType = "image/png";

export default function Image() {
  return ogImage({
    kicker: "M-Pesa STK Push API",
    title: SITE.tagline,
    subtitle: "One API call. Funds land in your bank account, PayBill or Till. Signed webhooks when they do.",
    code: "POST /v1/payments  →  201 pending  →  payment.succeeded",
  });
}
