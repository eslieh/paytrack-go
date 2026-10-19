"use client";

import { useEffect, useState } from "react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, SkeletonRows } from "@/components/ui.jsx";
import { Stars } from "@/components/support.jsx";

export default function FeedbackPage() {
  const [items, setItems] = useState(null);
  const [cursor, setCursor] = useState(null);
  const [error, setError] = useState(null);

  async function load(after) {
    setError(null);
    try {
      const r = await api.admin.support.feedback({ cursor: after });
      setItems((cur) => (after ? [...(cur || []), ...r.data] : r.data));
      setCursor(r.has_more ? r.next_cursor : null);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, []);

  return (
    <section className={styles.card}>
      <div className={styles.h2}>Feedback</div>
      <div className={styles.muted} style={{ fontSize: 13 }}>Product feedback sent from the dashboard, newest first.</div>
      <div style={{ display: "grid", gap: 10, marginTop: 12 }}>
        {!items && !error && <SkeletonRows rows={3} cols={1} />}
        {items?.length === 0 && <div className={styles.empty}>No feedback yet.</div>}
        {items?.map((f) => (
          <div key={f.id} style={{ border: "1px solid #e9ebf4", borderRadius: 12, padding: 14 }}>
            <div className={styles.rowBetween} style={{ gap: 8, flexWrap: "wrap", marginBottom: 6 }}>
              <span style={{ fontSize: 13 }}><strong>{f.user_email}</strong>{f.page && <span className={styles.muted}> · {f.page}</span>}</span>
              <span style={{ display: "inline-flex", gap: 10, alignItems: "center" }}>
                {f.rating && <Stars value={f.rating} size={14} />}
                <span className={styles.muted} style={{ fontSize: 12 }}>{fmtDate(f.created_at)}</span>
              </span>
            </div>
            <div style={{ whiteSpace: "pre-wrap", fontSize: 14, lineHeight: 1.6, overflowWrap: "anywhere" }}>{f.message}</div>
          </div>
        ))}
      </div>
      {cursor && <button className="btn" style={{ marginTop: 12 }} onClick={() => load(cursor)}>Load more</button>}
      <ErrorText error={error} />
    </section>
  );
}
