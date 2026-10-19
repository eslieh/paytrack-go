import { ogImage, OG_SIZE } from "@/lib/og";

export const alt = "Paytrack API Reference";
export const size = OG_SIZE;
export const contentType = "image/png";

export default function Image() {
  return ogImage({
    kicker: "API Reference",
    title: "Every endpoint, request and response.",
    subtitle: "Payments, channels, signed webhooks, errors and idempotency, with samples in cURL, Node.js, Python and PHP.",
    code: "curl -X POST /v1/payments -H \"Idempotency-Key: order-1042\"",
  });
}
