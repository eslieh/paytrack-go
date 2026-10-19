"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, KV, PageSkeleton, StatusBadge } from "@/components/ui.jsx";
import { ActionModal, AuditTrail, Section } from "@/components/admin.jsx";

// Recovery actions: what each does, shown in its confirmation.
const RECOVERY = {
  send_password_reset: { label: "Send password reset", confirm: "Send link", text: "Emails the user a link to set a new password (valid 30 minutes). You never see or set their password." },
  resend_verification: { label: "Resend verification", confirm: "Resend", text: "Emails a new verification link to the user's current address." },
  revoke_sessions: { label: "Sign out everywhere", confirm: "Sign out", text: "Ends every session and forgets trusted devices. The user is emailed and notified." },
  reset_two_step: { label: "Reset two-step", confirm: "Reset", text: "Turns email sign-in codes back on and forgets trusted devices, so the next sign-in needs a code." },
  unlink_google: { label: "Unlink Google", confirm: "Unlink", text: "Removes Google sign-in. Only possible if the user has a password." },
  clear_rate_limits: { label: "Clear sign-in limits", confirm: "Clear", text: "Lifts the temporary lock after too many sign-in, reset or verification attempts." },
};

export default function AdminUserPage({ params }) {
  const { id } = use(params);
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [modal, setModal] = useState(null);
  const [done, setDone] = useState("");
  const [version, setVersion] = useState(0);

  useEffect(() => { api.admin.user(id).then(setData).catch(setError); }, [id, version]);

  if (error && !data) return <section className={styles.card}><BackLink /><ErrorText error={error} /></section>;
  if (!data) return <PageSkeleton />;
  const u = data.user;
  const google = data.identities.find((i) => i.provider === "google");
  const close = (label) => (ok) => {
    setModal(null);
    if (ok) { setDone(`${label}: done.`); setVersion((v) => v + 1); }
  };

  const available = Object.keys(RECOVERY).filter((k) =>
    (k !== "resend_verification" || !u.email_verified) && (k !== "unlink_google" || google) && (k !== "send_password_reset" || !u.disabled_at));

  return (
    <div style={{ display: "grid", gap: 16, gridTemplateColumns: "minmax(0, 1fr)" }}>
      <section className={styles.card}>
        <BackLink tenantId={u.tenant_id} />
        <div className={styles.rowBetween} style={{ marginTop: 8, gap: 12, flexWrap: "wrap" }}>
          <div>
            <div className={styles.h2} style={{ wordBreak: "break-all" }}>{u.email}</div>
            <div className={styles.mono} style={{ letterSpacing: 0, fontSize: 12, color: "#657090" }}>{u.id}</div>
          </div>
          <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
            {u.disabled_at ? <StatusBadge status="disabled" tone="danger" /> : <StatusBadge status="active" tone="success" />}
            <StatusBadge status={u.email_verified ? "verified" : "unverified"} tone={u.email_verified ? "success" : "warning"} />
            <StatusBadge status={u.two_step_enabled ? "two-step on" : "two-step off"} tone={u.two_step_enabled ? "neutral" : "warning"} />
          </div>
        </div>
        {u.disabled_at && (
          <div className={styles.notice} style={{ marginTop: 12, background: "#fef3f2", borderColor: "#fecdca", color: "#912018" }}>
            Disabled {fmtDate(u.disabled_at)}: {u.disabled_reason}
          </div>
        )}
        <div style={{ marginTop: 12 }}>
          <KV label="Tenant"><Link href={`/admin/tenants/${u.tenant_id}`} className={styles.mono} style={{ letterSpacing: 0, fontSize: 12, color: "#3846f7" }}>{u.tenant_id}</Link></KV>
          <KV label="Role">{u.role}</KV>
          <KV label="Signs in with">{[u.has_password && "password", google && `Google (${google.email})`].filter(Boolean).join(", ") || "—"}</KV>
          <KV label="Created">{fmtDate(u.created_at)}</KV>
          <KV label="Last sign-in">{u.last_login_at ? fmtDate(u.last_login_at) : "Never"}</KV>
        </div>
      </section>

      <Section title="Account recovery">
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          {available.map((k) => <button key={k} className="btn" onClick={() => setModal(k)}>{RECOVERY[k].label}</button>)}
          <button className="btn" onClick={() => setModal("change_email")}>Change email</button>
          {u.disabled_at
            ? <button className="btn" onClick={() => setModal("enable")}>Re-enable sign-in</button>
            : <button className="btn" style={{ color: "#b42318" }} onClick={() => setModal("disable")}>Disable sign-in</button>}
        </div>
        {done && <div className={styles.notice} style={{ marginTop: 12 }}>{done}</div>}
      </Section>

      <Section title={`Active sessions (${data.sessions.length})`}>
        {data.sessions.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No active sessions.</div>}
        {data.sessions.map((s) => (
          <div key={s.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontSize: 13 }}><strong>{s.device}</strong> <span className={styles.muted}>· {s.ip} · {s.auth_method}</span></span>
            <span className={styles.muted} style={{ fontSize: 12 }}>last seen {fmtDate(s.last_seen_at)}</span>
          </div>
        ))}
      </Section>

      <Section title="Security activity">
        {data.security_events.length === 0 && <div className={styles.muted} style={{ fontSize: 13 }}>No activity.</div>}
        {data.security_events.map((e) => (
          <div key={e.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontSize: 13 }}><strong>{e.type.replaceAll("_", " ")}</strong> <span className={styles.muted}>· {e.device} · {e.ip}{e.data?.by === "support" ? " · by support" : ""}</span></span>
            <span className={styles.muted} style={{ fontSize: 12 }}>{fmtDate(e.created_at)}</span>
          </div>
        ))}
      </Section>

      <Section title="Staff actions"><AuditTrail targetType="user" targetId={u.id} refresh={version} /></Section>

      {RECOVERY[modal] && (
        <ActionModal title={`${RECOVERY[modal].label} for ${u.email}?`} confirmLabel={RECOVERY[modal].confirm} description={RECOVERY[modal].text}
          onConfirm={(reason) => api.admin.recover(u.id, modal, reason)} onClose={close(RECOVERY[modal].label)} />
      )}
      {modal === "change_email" && (
        <ActionModal title={`Change ${u.email}'s sign-in email?`} confirmLabel="Change email" extraField={{ label: "New email", type: "email" }}
          description="Verify the owner's identity first. The new address gets a verification link, every session ends, and the old address is warned."
          onConfirm={(reason, email) => api.admin.changeEmail(u.id, email, reason)} onClose={close("Email changed")} />
      )}
      {modal === "disable" && (
        <ActionModal title={`Disable sign-in for ${u.email}?`} confirmLabel="Disable" danger
          description="Ends every session and blocks sign-in, refresh and password resets until re-enabled. The user is emailed the reason."
          onConfirm={(reason) => api.admin.disableUser(u.id, reason)} onClose={close("Disabled")} />
      )}
      {modal === "enable" && (
        <ActionModal title={`Re-enable sign-in for ${u.email}?`} confirmLabel="Re-enable"
          onConfirm={(reason) => api.admin.enableUser(u.id, reason)} onClose={close("Re-enabled")} />
      )}
    </div>
  );
}

function BackLink({ tenantId }) {
  return (
    <Link href={tenantId ? `/admin/tenants/${tenantId}` : "/admin/tenants"} className="btn btn-ghost" style={{ padding: 0, fontSize: 13 }}>
      <ArrowLeft size={14} /> {tenantId ? "Tenant" : "Accounts"}
    </Link>
  );
}
