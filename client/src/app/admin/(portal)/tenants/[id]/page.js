"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { destinationLabel, fmtDate, fmtKES, statusTone } from "@/lib/format";
import { ErrorText, KV, PageSkeleton, StatusBadge } from "@/components/ui.jsx";
import { ActionModal, AuditTrail, Section } from "@/components/admin.jsx";
import { TicketStatus, ticketRef } from "@/components/support.jsx";

export default function AdminTenantPage({ params }) {
  const { id } = use(params);
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [related, setRelated] = useState({});
  const [modal, setModal] = useState(null);
  const [version, setVersion] = useState(0);

  useEffect(() => {
    api.admin.tenant(id).then(setData).catch(setError);
    const get = (key, p) => p.then((r) => setRelated((cur) => ({ ...cur, [key]: r.data }))).catch(() => setRelated((cur) => ({ ...cur, [key]: [] })));
    get("users", api.admin.users({ tenant_id: id }));
    get("payments", api.admin.payments({ tenant_id: id, limit: 10 }));
    get("deliveries", api.admin.deliveries({ tenant_id: id, limit: 10 }));
    get("tickets", api.admin.support.list({ tenant_id: id, limit: 10 }));
  }, [id, version]);

  if (error && !data) return <section className={styles.card}><BackLink /><ErrorText error={error} /></section>;
  if (!data) return <PageSkeleton />;
  const t = data.tenant;
  const suspended = t.status === "suspended";

  return (
    <div style={{ display: "grid", gap: 16, gridTemplateColumns: "minmax(0, 1fr)" }}>
      <section className={styles.card}>
        <BackLink />
        <div className={styles.rowBetween} style={{ marginTop: 8, gap: 12, flexWrap: "wrap" }}>
          <div>
            <div className={styles.h2}>{t.name}</div>
            <div className={styles.mono} style={{ letterSpacing: 0, fontSize: 12, color: "#657090" }}>{t.id}</div>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <StatusBadge status={t.status} tone={suspended ? "danger" : "success"} />
            {suspended
              ? <button className="btn" onClick={() => setModal("reinstate")}>Reinstate</button>
              : <button className="btn" style={{ color: "#b42318" }} onClick={() => setModal("suspend")}>Suspend</button>}
          </div>
        </div>
        {suspended && (
          <div className={styles.notice} style={{ marginTop: 12, background: "#fef3f2", borderColor: "#fecdca", color: "#912018" }}>
            Suspended {fmtDate(t.suspended_at)}: {t.suspended_reason}
          </div>
        )}
        <div style={{ marginTop: 12 }}>
          <KV label="Created">{fmtDate(t.created_at)}</KV>
          <KV label="Webhook URL"><span style={{ wordBreak: "break-all" }}>{t.webhook_url || "Not set"}</span></KV>
          <KV label="API keys">{data.api_keys.filter((k) => !k.revoked_at).length} active of {data.api_keys.length}</KV>
        </div>
      </section>

      <Section title="Users">
        {related.users?.map((u) => (
          <div key={u.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <Link href={`/admin/users/${u.id}`} style={{ fontWeight: 600, color: "#3846f7", wordBreak: "break-all" }}>{u.email}</Link>
            <span style={{ display: "flex", gap: 6 }}>
              <StatusBadge status={u.role} tone="neutral" />
              {u.disabled_at ? <StatusBadge status="disabled" tone="danger" /> : !u.email_verified && <StatusBadge status="unverified" tone="warning" />}
            </span>
          </div>
        ))}
      </Section>

      <Section title={`Channels (${data.channels.length})`}>
        {data.channels.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No channels.</div>}
        {data.channels.map((c) => (
          <div key={c.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontSize: 13 }}><strong>{c.name}</strong> <span className={styles.muted}>{destinationLabel(c)}</span></span>
            <StatusBadge status={c.active ? "active" : "inactive"} tone={c.active ? "success" : "neutral"} />
          </div>
        ))}
      </Section>

      <Section title="Recent payments">
        {related.payments?.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No payments.</div>}
        {related.payments?.map((p) => (
          <div key={p.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontSize: 13 }}><strong>{fmtKES(p.amount)}</strong> <span className={styles.mono} style={{ letterSpacing: 0, fontSize: 12, color: "#657090" }}>{p.id}</span></span>
            <span style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <span className={styles.muted} style={{ fontSize: 12 }}>{fmtDate(p.created_at)}</span>
              <StatusBadge status={p.status} tone={statusTone(p.status)} />
            </span>
          </div>
        ))}
      </Section>

      <Section title="Recent webhook deliveries">
        {related.deliveries?.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No deliveries.</div>}
        {related.deliveries?.map((d) => (
          <div key={d.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontSize: 13 }}>{d.event_type} <span className={styles.muted}>· {d.attempts} attempt{d.attempts === 1 ? "" : "s"}{d.last_error ? ` · ${d.last_error}` : ""}</span></span>
            <StatusBadge status={d.status} tone={statusTone(d.status)} />
          </div>
        ))}
      </Section>

      <Section title="Support tickets">
        {related.tickets?.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No tickets.</div>}
        {related.tickets?.map((tk) => (
          <Link key={tk.id} href={`/admin/support/${tk.id}`} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, color: "inherit", textDecoration: "none" }}>
            <span style={{ fontSize: 13 }}><span className={styles.mono} style={{ letterSpacing: 0 }}>{ticketRef(tk)}</span> {tk.subject}</span>
            <TicketStatus status={tk.status} side="staff" />
          </Link>
        ))}
      </Section>

      <Section title="Staff actions"><AuditTrail targetType="tenant" targetId={t.id} refresh={version} /></Section>

      {modal === "suspend" && (
        <ActionModal title={`Suspend ${t.name}?`} confirmLabel="Suspend" danger
          description="Their API keys stop working and the dashboard becomes read-only (Support stays open). Payments already in progress still complete. The owner is emailed the reason you give."
          onConfirm={(reason) => api.admin.suspendTenant(t.id, reason)}
          onClose={(done) => { setModal(null); if (done) setVersion((v) => v + 1); }} />
      )}
      {modal === "reinstate" && (
        <ActionModal title={`Reinstate ${t.name}?`} confirmLabel="Reinstate"
          description="API keys work again and the dashboard is fully usable. The owner is emailed."
          onConfirm={(reason) => api.admin.reinstateTenant(t.id, reason)}
          onClose={(done) => { setModal(null); if (done) setVersion((v) => v + 1); }} />
      )}
    </div>
  );
}

function BackLink() {
  return <Link href="/admin/tenants" className="btn btn-ghost" style={{ padding: 0, fontSize: 13 }}><ArrowLeft size={14} /> Accounts</Link>;
}

