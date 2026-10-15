// Display helpers shared by dashboard pages.

export const fmtKES = (n) =>
  `KES ${Number(n ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

export const fmtDate = (s) => (s ? new Date(s).toLocaleString() : "—");

export function maskPhone(p) {
  if (!p) return "—";
  return p.length > 7 ? `${p.slice(0, 6)}***${p.slice(-3)}` : p;
}

export function maskSecret(s, visible = 6) {
  if (!s) return "—";
  if (s.length <= visible + 4) return "•".repeat(s.length);
  return `${s.slice(0, visible)}${"•".repeat(Math.min(24, s.length - visible - 4))}${s.slice(-4)}`;
}

export const CHANNEL_TYPE_LABEL = { bank: "Bank account", paybill: "PayBill", till: "Till (Buy Goods)" };

// Where a channel or payment routes money, e.g. "Equity Bank · 0123456789", "PayBill 400200 · ACC1", "Till 123456".
export function destinationLabel({ type, destination_type, business_number, bank_name, account_number, account_reference }) {
  const t = type || destination_type;
  const acc = account_number || (t !== "till" ? account_reference : "");
  if (t === "bank") return [bank_name || `Bank ${business_number}`, acc].filter(Boolean).join(" · ");
  if (t === "paybill") return [`PayBill ${business_number}`, acc].filter(Boolean).join(" · ");
  if (t === "till") return `Till ${business_number}`;
  return business_number || "—";
}

// Badge tone for payment and delivery statuses.
export function statusTone(status) {
  switch (status) {
    case "succeeded":
    case "delivered":
      return "success";
    case "failed":
    case "cancelled":
    case "expired":
      return "danger";
    case "pending":
    case "delivering":
    case "created":
      return "warning";
    default:
      return "neutral";
  }
}

export function newIdempotencyKey() {
  if (typeof crypto !== "undefined" && crypto.randomUUID) return crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}

export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
