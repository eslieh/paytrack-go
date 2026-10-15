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
