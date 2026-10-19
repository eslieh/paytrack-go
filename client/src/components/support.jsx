"use client";

import { useState } from "react";
import { Star } from "lucide-react";
import styles from "../app/dashboard/styles.module.css";
import { fmtDate } from "../lib/format";
import { StatusBadge } from "./ui.jsx";
import { Busy } from "./Busy.jsx";

export const CATEGORIES = [
  ["payments", "Payments"],
  ["channels", "Channels"],
  ["webhooks", "Webhooks"],
  ["api", "API & integration"],
  ["account", "Account & sign-in"],
  ["billing", "Billing"],
  ["other", "Something else"],
];

export const categoryLabel = (c) => CATEGORIES.find(([k]) => k === c)?.[1] || c;

// Customers and staff see the same status from opposite sides.
const STATUS = {
  awaiting_staff: { user: "Waiting on Paytrack", staff: "Needs reply", tone: "warning" },
  awaiting_user: { user: "Awaiting your reply", staff: "Waiting on customer", tone: "neutral" },
  resolved: { user: "Resolved", staff: "Resolved", tone: "success" },
  closed: { user: "Closed", staff: "Closed", tone: "neutral" },
};

export function TicketStatus({ status, side = "user" }) {
  const s = STATUS[status] || { user: status, staff: status, tone: "neutral" };
  return <StatusBadge status={s[side]} tone={s.tone} />;
}

export const ticketRef = (t) => `#${t.number}`;

// Thread renders a ticket's messages oldest first. Internal notes (staff
// only) are tinted so they can't be mistaken for replies.
export function Thread({ messages, side = "user" }) {
  return (
    <div style={{ display: "grid", gap: 12, marginTop: 12 }}>
      {messages.map((m) => {
        const mine = (side === "user") === (m.author_kind === "user");
        const bg = m.internal ? "#fffaeb" : mine ? "#f5f6ff" : "#fff";
        const border = m.internal ? "#fedf89" : "#e9ebf4";
        return (
          <div key={m.id} style={{ border: `1px solid ${border}`, background: bg, borderRadius: 12, padding: 14 }}>
            <div className={styles.rowBetween} style={{ marginBottom: 6, gap: 8, flexWrap: "wrap" }}>
              <strong style={{ fontSize: 13 }}>
                {m.author_kind === "staff" && side === "staff" ? (m.internal ? "Internal note" : "Paytrack Support (you)") : m.author_name}
              </strong>
              <span className={styles.muted} style={{ fontSize: 12 }}>{fmtDate(m.created_at)}</span>
            </div>
            <div style={{ whiteSpace: "pre-wrap", fontSize: 14, lineHeight: 1.6, overflowWrap: "anywhere" }}>{m.body}</div>
          </div>
        );
      })}
    </div>
  );
}

// ReplyBox posts a message; onSend returns a promise.
export function ReplyBox({ onSend, placeholder = "Write a reply…", label = "Send reply", children, disabled }) {
  const [body, setBody] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState(null);

  async function submit(e) {
    e.preventDefault();
    if (!body.trim()) return;
    setSending(true);
    setError(null);
    try {
      await onSend(body.trim());
      setBody("");
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  }

  return (
    <form onSubmit={submit} style={{ marginTop: 16, display: "grid", gap: 10 }}>
      <textarea className="input" rows={5} style={{ resize: "vertical", minHeight: 110, fontFamily: "inherit" }}
        placeholder={placeholder} value={body} onChange={(e) => setBody(e.target.value)} disabled={disabled || sending} maxLength={10000} />
      {error && <div className={styles.error}>{error}</div>}
      <div className={styles.rowBetween} style={{ gap: 12, flexWrap: "wrap" }}>
        <div>{children}</div>
        <button className="btn btn-primary" type="submit" disabled={disabled || sending || !body.trim()}>
          {sending ? <Busy>Sending…</Busy> : label}
        </button>
      </div>
    </form>
  );
}

export function Stars({ value, onChange, size = 22 }) {
  return (
    <div style={{ display: "flex", gap: 4 }} role="radiogroup" aria-label="Rating">
      {[1, 2, 3, 4, 5].map((n) => (
        <button key={n} type="button" role="radio" aria-checked={value === n} aria-label={`${n} star${n > 1 ? "s" : ""}`}
          onClick={onChange ? () => onChange(n) : undefined} disabled={!onChange}
          style={{ background: "none", border: 0, padding: 2, cursor: onChange ? "pointer" : "default" }}>
          <Star size={size} color={n <= (value || 0) ? "#f5a524" : "#c9cee0"} fill={n <= (value || 0) ? "#f5a524" : "none"} />
        </button>
      ))}
    </div>
  );
}
