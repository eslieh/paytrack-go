"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, SkeletonRows, StatusBadge } from "@/components/ui.jsx";
import { TicketStatus, categoryLabel, ticketRef } from "@/components/support.jsx";

const TABS = [
  ["awaiting_staff", "Needs reply"],
  ["awaiting_user", "Waiting on customer"],
  ["resolved", "Resolved"],
  ["closed", "Closed"],
  ["", "All"],
];

const PRIORITY_TONE = { urgent: "danger", high: "warning", normal: "neutral", low: "neutral" };

export default function SupportQueuePage() {
  const [status, setStatus] = useState("awaiting_staff");
  const [counts, setCounts] = useState({});
  const [tickets, setTickets] = useState(null);
  const [cursor, setCursor] = useState(null);
  const [error, setError] = useState(null);

  async function load(after) {
    setError(null);
    try {
      const r = await api.admin.support.list({ status, cursor: after });
      setTickets((cur) => (after ? [...(cur || []), ...r.data] : r.data));
      setCursor(r.has_more ? r.next_cursor : null);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { setTickets(null); load(); }, [status]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { api.admin.support.summary().then((r) => setCounts(r.counts)).catch(() => {}); }, []);

  return (
    <section className={styles.card}>
      <div className={styles.h2}>Support</div>
      <div style={{ display: "flex", gap: 6, flexWrap: "wrap", margin: "12px 0 4px" }}>
        {TABS.map(([key, label]) => (
          <button key={key || "all"} className={`btn ${status === key ? "btn-primary" : ""}`} style={{ padding: "6px 12px", fontSize: 13 }} onClick={() => setStatus(key)}>
            {label}{key && counts[key] != null ? ` · ${counts[key]}` : ""}
          </button>
        ))}
      </div>

      <div className={styles.table} style={{ "--cols": "0.6fr 2.2fr 1.6fr 0.9fr 0.8fr 1.2fr 1.2fr", "--min-w": "900px" }}>
        <div className={styles.thead}>
          <div>#</div><div>Subject</div><div>Customer</div><div>Topic</div><div>Priority</div><div>Status</div><div>Last message</div>
        </div>
        <div className={styles.tbody}>
          {!tickets && !error && <SkeletonRows rows={3} cols={7} />}
          {tickets?.length === 0 && <div className={styles.empty}>Nothing here. Inbox zero.</div>}
          {tickets?.map((t) => (
            <Link key={t.id} href={`/admin/support/${t.id}`} className={styles.trow} style={{ color: "inherit", textDecoration: "none" }}>
              <div className={styles.mono} style={{ letterSpacing: 0 }}>{ticketRef(t)}</div>
              <div className={styles.cellTitle}>{t.subject}</div>
              <div>
                <div style={{ fontSize: 13, fontWeight: 600 }}>{t.tenant_name}</div>
                <div className={styles.cellSub} style={{ wordBreak: "break-all" }}>{t.requester_email}</div>
              </div>
              <div className={styles.cellSub}>{categoryLabel(t.category)}</div>
              <div><StatusBadge status={t.priority} tone={PRIORITY_TONE[t.priority]} /></div>
              <div><TicketStatus status={t.status} side="staff" /></div>
              <div className={styles.cellSub}>{fmtDate(t.last_message_at)}</div>
            </Link>
          ))}
        </div>
      </div>
      {cursor && <button className="btn" style={{ marginTop: 12 }} onClick={() => load(cursor)}>Load more</button>}
      <ErrorText error={error} />
    </section>
  );
}
