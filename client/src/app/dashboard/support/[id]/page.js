"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, PageSkeleton } from "@/components/ui.jsx";
import { Busy } from "@/components/Busy.jsx";
import { ReplyBox, Stars, Thread, TicketStatus, categoryLabel, ticketRef } from "@/components/support.jsx";

export default function TicketPage({ params }) {
  const { id } = use(params);
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);

  async function load() {
    try {
      setData(await api.support.get(id));
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, [id]); // eslint-disable-line react-hooks/exhaustive-deps

  if (error && !data) return <section className={styles.card}><BackLink /><ErrorText error={error} /></section>;
  if (!data) return <PageSkeleton />;
  const t = data.ticket;
  const closed = t.status === "closed";

  return (
    <>
      <section className={styles.card}>
        <BackLink />
        <div className={styles.rowBetween} style={{ marginTop: 8, gap: 12, flexWrap: "wrap" }}>
          <div>
            <div className={styles.h2}>{t.subject}</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>{ticketRef(t)} · {categoryLabel(t.category)} · opened {fmtDate(t.created_at)}</div>
          </div>
          <TicketStatus status={t.status} />
        </div>

        <Thread messages={data.messages} />

        {closed ? (
          <div className={styles.notice} style={{ marginTop: 16 }}>This ticket is closed. <Link href="/dashboard/support" style={{ color: "#6472f3" }}>Open a new ticket</Link> if you need more help.</div>
        ) : (
          <ReplyBox
            placeholder={t.status === "resolved" ? "Still need help? Replying reopens this ticket." : "Write a reply…"}
            onSend={async (body) => { await api.support.reply(t.id, body); await load(); }}
          />
        )}
      </section>

      {(t.status === "resolved" || closed) && <RateCard ticket={t} onRated={(nt) => setData((d) => ({ ...d, ticket: nt }))} />}
    </>
  );
}

function BackLink() {
  return (
    <Link href="/dashboard/support" className="btn btn-ghost" style={{ padding: 0, fontSize: 13 }}>
      <ArrowLeft size={14} /> All tickets
    </Link>
  );
}

function RateCard({ ticket, onRated }) {
  const [rating, setRating] = useState(ticket.rating);
  const [comment, setComment] = useState(ticket.rating_comment || "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const rated = ticket.rating != null;

  async function submit(e) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      onRated(await api.support.rate(ticket.id, rating, comment.trim()));
    } catch (err) {
      setError(err);
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className={styles.card} id="rate">
      <div style={{ fontWeight: 700 }}>{rated ? "Thanks for your rating" : "How did we do?"}</div>
      <div className={styles.muted} style={{ fontSize: 13, marginBottom: 10 }}>
        {rated ? "You can update it any time." : "Rate the help you got on this ticket."}
      </div>
      <form onSubmit={submit} style={{ display: "grid", gap: 10 }}>
        <Stars value={rating} onChange={setRating} />
        <input className="input" placeholder="Anything to add? (optional)" value={comment} onChange={(e) => setComment(e.target.value)} maxLength={1000} />
        <ErrorText error={error} />
        <div><button className="btn" type="submit" disabled={!rating || saving}>{saving ? <Busy>Saving…</Busy> : rated ? "Update rating" : "Submit rating"}</button></div>
      </form>
    </section>
  );
}
