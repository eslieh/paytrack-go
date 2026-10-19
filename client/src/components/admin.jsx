"use client";

import { useEffect, useState } from "react";
import styles from "../app/dashboard/styles.module.css";
import { api } from "../lib/api";
import { fmtDate } from "../lib/format";
import { ErrorText, Field, Modal, SkeletonRows } from "./ui.jsx";
import { Busy } from "./Busy.jsx";

// ActionModal confirms an account action. Every action needs a reason: it is
// written to the audit log (and, for suspensions, shown to the customer).
export function ActionModal({ title, description, confirmLabel, danger, extraField, onConfirm, onClose }) {
  const [reason, setReason] = useState("");
  const [extra, setExtra] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onConfirm(reason.trim(), extra.trim());
      onClose(true);
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return (
    <Modal title={title} onClose={() => onClose(false)}>
      <form onSubmit={submit} style={{ display: "grid", gap: 12 }}>
        {description && <div className={styles.muted} style={{ fontSize: 13 }}>{description}</div>}
        {extraField && (
          <Field label={extraField.label}>
            <input className="input" type={extraField.type || "text"} value={extra} onChange={(e) => setExtra(e.target.value)} required autoFocus />
          </Field>
        )}
        <Field label="Reason" hint="5–500 characters. Recorded in the audit log.">
          <textarea className="input" rows={3} style={{ resize: "vertical", fontFamily: "inherit" }} value={reason}
            onChange={(e) => setReason(e.target.value)} minLength={5} maxLength={500} required autoFocus={!extraField} />
        </Field>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={() => onClose(false)} disabled={busy}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={busy}
            style={danger ? { background: "#b42318", boxShadow: "none" } : undefined}>
            {busy ? <Busy>Working…</Busy> : confirmLabel}
          </button>
        </div>
      </form>
    </Modal>
  );
}

const ACTION_LABEL = {
  "staff.sign_in": "Signed in",
  "tenant.suspend": "Suspended tenant",
  "tenant.reinstate": "Reinstated tenant",
  "user.disable": "Disabled user",
  "user.enable": "Re-enabled user",
  "user.change_email": "Changed email",
  "user.send_password_reset": "Sent password reset",
  "user.resend_verification": "Resent verification",
  "user.revoke_sessions": "Signed out everywhere",
  "user.reset_two_step": "Reset two-step",
  "user.unlink_google": "Unlinked Google",
  "user.clear_rate_limits": "Cleared sign-in limits",
};
export const actionLabel = (a) => ACTION_LABEL[a] || a;

// AuditTrail lists audit entries for one tenant or user.
export function AuditTrail({ targetType, targetId, refresh }) {
  const [entries, setEntries] = useState(null);
  const [error, setError] = useState(null);
  useEffect(() => {
    api.admin.auditLog({ target_type: targetType, target_id: targetId, limit: 50 })
      .then((r) => setEntries(r.data)).catch(setError);
  }, [targetType, targetId, refresh]);
  return (
    <>
      {!entries && !error && <SkeletonRows rows={2} cols={1} />}
      {entries?.length === 0 && <div className={styles.empty}>No staff actions yet.</div>}
      {entries?.map((e) => (
        <div key={e.id} className={styles.rowBetween} style={{ padding: "8px 0", borderTop: "1px solid #f1f3fb", gap: 12, flexWrap: "wrap" }}>
          <span style={{ fontSize: 13 }}><strong>{actionLabel(e.action)}</strong>{e.reason && <span className={styles.muted}>: {e.reason}</span>}</span>
          <span className={styles.muted} style={{ fontSize: 12 }}>{fmtDate(e.created_at)}</span>
        </div>
      ))}
      <ErrorText error={error} />
    </>
  );
}

export function Section({ title, right, children }) {
  return (
    <section className={styles.card}>
      <div className={styles.rowBetween} style={{ marginBottom: 8, gap: 12, flexWrap: "wrap" }}>
        <div style={{ fontWeight: 700 }}>{title}</div>
        {right}
      </div>
      {children}
    </section>
  );
}
