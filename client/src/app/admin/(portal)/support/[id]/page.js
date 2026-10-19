"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { CopyButton, ErrorText, KV, PageSkeleton } from "@/components/ui.jsx";
import { ReplyBox, Stars, Thread, TicketStatus, categoryLabel, ticketRef } from "@/components/support.jsx";

const STATUSES = [
  ["awaiting_staff", "Needs reply"],
  ["awaiting_user", "Waiting on customer"],
  ["resolved", "Resolved (emails customer)"],
  ["closed", "Closed"],
];
const PRIORITIES = ["low", "normal", "high", "urgent"];

export default function AdminTicketPage({ params }) {
  const { id } = use(params);
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [internal, setInternal] = useState(false);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setData(await api.admin.support.get(id));
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, [id]); // eslint-disable-line react-hooks/exhaustive-deps

  async function update(patch) {
    setSaving(true);
    setError(null);
    try {
      const ticket = await api.admin.support.update(id, patch);
      setData((d) => ({ ...d, ticket }));
    } catch (e) {
      setError(e);
    } finally {
      setSaving(false);
    }
  }

  if (error && !data) return <section className={styles.card}><BackLink /><ErrorText error={error} /></section>;
  if (!data) return <PageSkeleton />;
  const t = data.ticket;

  return (
    <div style={{ display: "grid", gap: 16, gridTemplateColumns: "minmax(0, 1fr)" }}>
      <section className={styles.card}>
        <BackLink />
        <div className={styles.rowBetween} style={{ marginTop: 8, gap: 12, flexWrap: "wrap" }}>
          <div>
            <div className={styles.h2}>{t.subject}</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>{ticketRef(t)} · {categoryLabel(t.category)} · opened {fmtDate(t.created_at)}</div>
          </div>
          <TicketStatus status={t.status} side="staff" />
        </div>

        <Thread messages={data.messages} side="staff" />

        <ReplyBox
          key={internal ? "note" : "reply"}
          label={internal ? "Add internal note" : "Send reply"}
          placeholder={internal ? "Only staff can see this note." : `Reply to ${t.requester_email}. They'll get an email with an excerpt and a link.`}
          onSend={async (body) => { await api.admin.support.reply(t.id, body, internal); await load(); }}
        >
          <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 13, cursor: "pointer" }}>
            <input type="checkbox" checked={internal} onChange={(e) => setInternal(e.target.checked)} />
            Internal note (not sent to customer)
          </label>
        </ReplyBox>
      </section>

      <section className={styles.card}>
        <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))" }}>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Status</span>
            <select className={styles.select} value={t.status} disabled={saving} onChange={(e) => update({ status: e.target.value })}>
              {STATUSES.map(([k, label]) => <option key={k} value={k}>{label}</option>)}
            </select>
          </label>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Priority</span>
            <select className={styles.select} value={t.priority} disabled={saving} onChange={(e) => update({ priority: e.target.value })}>
              {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
          </label>
        </div>
        <ErrorText error={error} />
        <div style={{ marginTop: 16 }}>
          <KV label="Business">{t.tenant_name}</KV>
          <KV label="Tenant ID"><span className={styles.mono} style={{ letterSpacing: 0, fontSize: 12 }}>{t.tenant_id}</span> <CopyButton text={t.tenant_id} label="" /></KV>
          <KV label="Requester">{t.requester_email}</KV>
          <KV label="User ID"><span className={styles.mono} style={{ letterSpacing: 0, fontSize: 12 }}>{t.user_id}</span> <CopyButton text={t.user_id} label="" /></KV>
          <KV label="First response">{t.first_response_at ? fmtDate(t.first_response_at) : "Not yet"}</KV>
          <KV label="Resolved">{t.resolved_at ? fmtDate(t.resolved_at) : "—"}</KV>
          <KV label="Rating">
            {t.rating ? <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Stars value={t.rating} size={14} />{t.rating_comment && <span className={styles.muted}>“{t.rating_comment}”</span>}</span> : "—"}
          </KV>
        </div>
      </section>
    </div>
  );
}

function BackLink() {
  return (
    <Link href="/admin/support" className="btn btn-ghost" style={{ padding: 0, fontSize: 13 }}>
      <ArrowLeft size={14} /> Queue
    </Link>
  );
}
