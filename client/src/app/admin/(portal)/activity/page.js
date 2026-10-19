"use client";

import { useEffect, useState } from "react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, SkeletonRows, StatusBadge } from "@/components/ui.jsx";
import { Busy } from "@/components/Busy.jsx";
import { actionLabel } from "@/components/admin.jsx";

const PAGE = 50;

const ACTION_TONE = {
  "tenant.suspend": "danger",
  "user.disable": "danger",
  "user.change_email": "warning",
  "tenant.reinstate": "success",
  "user.enable": "success",
};

// Activity is the admin audit log: every staff sign-in and account action.
export default function ActivityPage() {
  const [entries, setEntries] = useState(null);
  const [more, setMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  async function load(before) {
    setLoading(true);
    setError(null);
    try {
      const { data } = await api.admin.auditLog({ limit: PAGE, before });
      setEntries((cur) => (before ? [...(cur || []), ...data] : data));
      setMore(data.length === PAGE);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  return (
    <section className={styles.card}>
      <div className={styles.h2}>Activity</div>
      <div className={styles.muted} style={{ fontSize: 13 }}>Every staff sign-in and account action, newest first.</div>

      <div className={styles.table} style={{ "--cols": "1.2fr 1.3fr 2fr 1fr 1.2fr", "--min-w": "760px" }}>
        <div className={styles.thead}>
          <div>Action</div><div>Target</div><div>Reason</div><div>IP</div><div>When</div>
        </div>
        <div className={styles.tbody}>
          {!entries && !error && <SkeletonRows rows={4} cols={5} />}
          {entries?.length === 0 && <div className={styles.empty}>No activity yet.</div>}
          {entries?.map((e) => (
            <div key={e.id} className={styles.trow}>
              <div><StatusBadge status={actionLabel(e.action)} tone={ACTION_TONE[e.action] || "neutral"} /></div>
              <div className={styles.mono} style={{ fontSize: 12, letterSpacing: 0, wordBreak: "break-all" }}>
                {e.target_type === "staff" ? "—" : e.target_id}
              </div>
              <div className={styles.cellSub}>{e.reason || "—"}</div>
              <div className={styles.mono} style={{ fontSize: 12, letterSpacing: 0 }}>{e.ip || "—"}</div>
              <div className={styles.cellSub}>{fmtDate(e.created_at)}</div>
            </div>
          ))}
        </div>
      </div>
      {more && (
        <button className="btn" style={{ marginTop: 12 }} disabled={loading} onClick={() => load(entries[entries.length - 1].id)}>
          {loading ? <Busy>Loading…</Busy> : "Load more"}
        </button>
      )}
      <ErrorText error={error} />
    </section>
  );
}
