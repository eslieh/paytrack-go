"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { Plus } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, Field, Modal, SkeletonRows } from "@/components/ui.jsx";
import { Busy } from "@/components/Busy.jsx";
import { CATEGORIES, Stars, TicketStatus, ticketRef } from "@/components/support.jsx";

export default function SupportPage() {
  const [tickets, setTickets] = useState(null);
  const [cursor, setCursor] = useState(null);
  const [error, setError] = useState(null);
  const [creating, setCreating] = useState(false);

  async function load(after) {
    setError(null);
    try {
      const r = await api.support.list({ cursor: after });
      setTickets((cur) => (after ? [...(cur || []), ...r.data] : r.data));
      setCursor(r.has_more ? r.next_cursor : null);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, []);

  return (
    <>
      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <div className={styles.h2}>Support</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>Ask us anything about payments, channels, webhooks or your account. We&apos;ll email you when we reply.</div>
          </div>
          <button className="btn btn-primary" onClick={() => setCreating(true)}><Plus size={16} /> New ticket</button>
        </div>

        <div className={styles.table} style={{ "--cols": "0.6fr 2.4fr 1.3fr 1.2fr", "--min-w": "560px" }}>
          <div className={styles.thead}>
            <div>Ticket</div><div>Subject</div><div>Status</div><div>Last update</div>
          </div>
          <div className={styles.tbody}>
            {!tickets && !error && <SkeletonRows rows={2} cols={4} />}
            {tickets?.length === 0 && <div className={styles.empty}>No tickets yet. Need help? Open one and we&apos;ll get back to you.</div>}
            {tickets?.map((t) => (
              <Link key={t.id} href={`/dashboard/support/${t.id}`} className={styles.trow} style={{ color: "inherit", textDecoration: "none" }}>
                <div className={styles.mono} style={{ letterSpacing: 0 }}>{ticketRef(t)}</div>
                <div className={styles.cellTitle}>{t.subject}</div>
                <div><TicketStatus status={t.status} /></div>
                <div className={styles.cellSub}>{fmtDate(t.last_message_at)}</div>
              </Link>
            ))}
          </div>
        </div>
        {cursor && <button className="btn" style={{ marginTop: 12 }} onClick={() => load(cursor)}>Load more</button>}
        <ErrorText error={error} />
      </section>

      <FeedbackCard />

      {creating && (
        <NewTicketModal
          onClose={() => setCreating(false)}
          onCreated={(t) => { setCreating(false); setTickets((cur) => [t, ...(cur || [])]); }}
        />
      )}
    </>
  );
}

function NewTicketModal({ onClose, onCreated }) {
  const [form, setForm] = useState({ subject: "", category: "payments", body: "" });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  async function submit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      onCreated(await api.support.create({ ...form, subject: form.subject.trim(), body: form.body.trim() }));
    } catch (err) {
      setError(err);
      setSubmitting(false);
    }
  }

  return (
    <Modal title="New support ticket" onClose={onClose}>
      <form onSubmit={submit} style={{ display: "grid", gap: 12 }}>
        <Field label="Topic">
          <select className={styles.select} value={form.category} onChange={set("category")}>
            {CATEGORIES.map(([k, label]) => <option key={k} value={k}>{label}</option>)}
          </select>
        </Field>
        <Field label="Subject">
          <input className="input" autoFocus value={form.subject} onChange={set("subject")} minLength={3} maxLength={150} required placeholder="e.g. Payment stuck in pending" />
        </Field>
        <Field label="Message" hint="Include payment IDs, channel names or request IDs if you have them. Never share your API keys.">
          <textarea className="input" rows={6} style={{ resize: "vertical", fontFamily: "inherit" }} value={form.body} onChange={set("body")} maxLength={10000} required />
        </Field>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>{submitting ? <Busy>Sending…</Busy> : "Send"}</button>
        </div>
      </form>
    </Modal>
  );
}

function FeedbackCard() {
  const pathname = usePathname();
  const [rating, setRating] = useState(null);
  const [message, setMessage] = useState("");
  const [state, setState] = useState("idle");
  const [error, setError] = useState(null);

  async function submit(e) {
    e.preventDefault();
    setState("sending");
    setError(null);
    try {
      await api.support.feedback({ rating, message: message.trim(), page: pathname });
      setState("sent");
      setMessage("");
      setRating(null);
    } catch (err) {
      setError(err);
      setState("idle");
    }
  }

  return (
    <section className={styles.card}>
      <div style={{ fontWeight: 700 }}>Share feedback</div>
      <div className={styles.muted} style={{ fontSize: 13, marginBottom: 12 }}>Not a problem, just a thought? Tell us what to improve. We read everything.</div>
      {state === "sent" ? (
        <div className={styles.notice}>Thanks! Your feedback went straight to the Paytrack team. <button className="btn btn-ghost" style={{ padding: 0 }} onClick={() => setState("idle")}>Send more</button></div>
      ) : (
        <form onSubmit={submit} style={{ display: "grid", gap: 10 }}>
          <Stars value={rating} onChange={setRating} />
          <textarea className="input" rows={3} style={{ resize: "vertical", fontFamily: "inherit" }} placeholder="What's working, what isn't, what you wish Paytrack did…" value={message} onChange={(e) => setMessage(e.target.value)} maxLength={5000} required />
          <ErrorText error={error} />
          <div><button className="btn" type="submit" disabled={state === "sending" || !message.trim()}>{state === "sending" ? <Busy>Sending…</Busy> : "Send feedback"}</button></div>
        </form>
      )}
    </section>
  );
}
