"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Bell, KeyRound, LogOut, Monitor, Send, ShieldCheck, User } from "lucide-react";
import styles from "../styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { useAuth } from "@/context/AuthContext.jsx";
import { ErrorText, Field, KV, Modal } from "@/components/ui.jsx";
import { AUTH_ERRORS, PasswordInput, StrengthMeter } from "@/components/auth.jsx";

export default function SettingsPage() {
  return (
    <Suspense fallback={null}>
      <Settings />
    </Suspense>
  );
}

function Settings() {
  const params = useSearchParams();
  const [me, setMe] = useState(null);
  const [error, setError] = useState(null);
  const { reload } = useAuth();

  const load = useCallback(async () => {
    try { setMe(await reload()); } catch (e) { setError(e); }
  }, [reload]);
  useEffect(() => { load(); }, [load]);

  const googleNotice = params.get("google") === "linked"
    ? { ok: true, text: "Google sign-in is now linked to your account." }
    : params.get("google_error")
      ? { ok: false, text: AUTH_ERRORS[params.get("google_error")] || "Linking Google failed." }
      : null;

  return (
    <>
      <section className={styles.card}>
        <div className={styles.kicker}>Settings</div>
        <h2 className={styles.h2}>Account &amp; notifications</h2>
        <ErrorText error={error} />
        {googleNotice && (
          <div className={styles.notice} style={{ marginTop: 10, ...(googleNotice.ok ? {} : { background: "#fef3f2", borderColor: "#fecdca", color: "#b42318" }) }}>
            {googleNotice.text}
          </div>
        )}
      </section>
      {me && <ProfileCard me={me} />}
      {me && <SecurityCard me={me} onChange={load} />}
      <NotificationsCard />
    </>
  );
}

function CardTitle({ icon: Icon, title, subtitle }) {
  return (
    <div style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 8 }}>
      <div style={{ width: 32, height: 32, borderRadius: 10, background: "#eef0ff", display: "grid", placeItems: "center", flex: "none" }}>
        <Icon size={18} color="#6472f3" />
      </div>
      <div>
        <div style={{ fontWeight: 700 }}>{title}</div>
        {subtitle && <div className={styles.muted} style={{ fontSize: 13 }}>{subtitle}</div>}
      </div>
    </div>
  );
}

function Toggle({ checked, onChange, disabled, label }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      style={{
        width: 42, height: 24, borderRadius: 999, border: 0, padding: 2, cursor: disabled ? "default" : "pointer",
        background: checked ? "#6472f3" : "#d0d5e6", transition: "background .2s", flex: "none",
      }}
    >
      <span style={{ display: "block", width: 20, height: 20, borderRadius: "50%", background: "#fff", transform: `translateX(${checked ? 18 : 0}px)`, transition: "transform .2s", boxShadow: "0 1px 3px rgba(0,0,0,.2)" }} />
    </button>
  );
}

function SettingRow({ title, description, children }) {
  return (
    <div className={styles.kv} style={{ alignItems: "center", padding: "12px 0" }}>
      <span style={{ display: "grid", gap: 2 }}>
        <span style={{ fontWeight: 600 }}>{title}</span>
        {description && <span className={styles.muted} style={{ fontSize: 13 }}>{description}</span>}
      </span>
      <span style={{ display: "flex", gap: 8, alignItems: "center", justifyContent: "flex-end" }}>{children}</span>
    </div>
  );
}

// ---------------------------------------------------------------- profile

function ProfileCard({ me }) {
  const u = me.user;
  return (
    <section className={styles.card}>
      <CardTitle icon={User} title="Profile" />
      <KV label="Business">{me.tenant.name}</KV>
      <KV label="Email">
        {u.email}{" "}
        <span className={u.email_verified ? styles.badgeSuccess : styles.badgeWarning}>{u.email_verified ? "verified" : "unverified"}</span>
      </KV>
      <KV label="Role">{u.role}</KV>
      <KV label="Member since">{fmtDate(u.created_at)}</KV>
    </section>
  );
}

// ---------------------------------------------------------------- security

function SecurityCard({ me, onChange }) {
  const u = me.user;
  const google = me.identities?.find((i) => i.provider === "google");
  const [modal, setModal] = useState(null); // "password" | "disable2fa"
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  async function enableTwoStep() {
    setBusy(true);
    setError(null);
    try { await api.account.setTwoStep(true, ""); await onChange(); } catch (e) { setError(e); } finally { setBusy(false); }
  }

  async function linkGoogle() {
    setError(null);
    try { window.location.href = (await api.account.linkGoogle()).url; } catch (e) { setError(e); }
  }

  async function unlinkGoogle() {
    setBusy(true);
    setError(null);
    try { await api.account.unlinkGoogle(); await onChange(); } catch (e) { setError(e); } finally { setBusy(false); }
  }

  return (
    <section className={styles.card}>
      <CardTitle icon={ShieldCheck} title="Security" subtitle="How you sign in to Paytrack" />
      <SettingRow title="Password" description={u.has_password ? "Changing it signs out your other sessions." : "You sign in with Google. Add a password as a backup."}>
        <button className="btn" style={{ border: "1px solid #e5e7ef", padding: "6px 12px" }} onClick={() => setModal("password")}>
          {u.has_password ? "Change" : "Set password"}
        </button>
      </SettingRow>
      <SettingRow
        title="Two-step sign-in"
        description={u.has_password ? "Email a 6-digit code when signing in with your password on a new device." : "Applies to password sign-ins once you set a password."}
      >
        <Toggle
          label="Two-step sign-in"
          checked={u.two_step_enabled}
          disabled={busy}
          onChange={(on) => (on ? enableTwoStep() : setModal("disable2fa"))}
        />
      </SettingRow>
      {me.google_enabled && (
        <SettingRow title="Google" description={google ? `Linked to ${google.email}` : "Sign in with your Google account."}>
          {google ? (
            <button className="btn" style={{ border: "1px solid #e5e7ef", padding: "6px 12px" }} disabled={busy || !u.has_password} title={u.has_password ? "" : "Set a password first"} onClick={unlinkGoogle}>Unlink</button>
          ) : (
            <button className="btn" style={{ border: "1px solid #e5e7ef", padding: "6px 12px" }} onClick={linkGoogle}>Link Google</button>
          )}
        </SettingRow>
      )}
      <ErrorText error={error} />
      <SessionsList />
      <ActivityList />

      {modal === "password" && <PasswordModal hasPassword={u.has_password} onClose={() => setModal(null)} onDone={onChange} />}
      {modal === "disable2fa" && <DisableTwoStepModal hasPassword={u.has_password} onClose={() => setModal(null)} onDone={onChange} />}
    </section>
  );
}

function PasswordModal({ hasPassword, onClose, onDone }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try { await api.account.changePassword(current, next); setDone(true); onDone(); } catch (err) { setError(err); } finally { setBusy(false); }
  }

  return (
    <Modal title={hasPassword ? "Change password" : "Set a password"} onClose={onClose}
      footer={done ? <button className="btn btn-primary" onClick={onClose}>Done</button> : null}>
      {done ? (
        <div className={styles.notice}>Password saved. Your other sessions were signed out and we emailed you a confirmation.</div>
      ) : (
        <form className="stack" onSubmit={submit}>
          {hasPassword && (
            <Field label="Current password">
              <div style={{ position: "relative" }}><PasswordInput id="current" value={current} onChange={setCurrent} /></div>
            </Field>
          )}
          <Field label="New password">
            <div style={{ position: "relative" }}><PasswordInput id="new" value={next} onChange={setNext} autoComplete="new-password" minLength={10} placeholder="At least 10 characters" /></div>
          </Field>
          <StrengthMeter password={next} />
          <ErrorText error={error} />
          <div className={styles.modalFooter}>
            <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button className="btn btn-primary" disabled={busy}>{busy ? "Saving..." : "Save password"}</button>
          </div>
        </form>
      )}
    </Modal>
  );
}

function DisableTwoStepModal({ hasPassword, onClose, onDone }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try { await api.account.setTwoStep(false, password); await onDone(); onClose(); } catch (err) { setError(err); } finally { setBusy(false); }
  }

  return (
    <Modal title="Turn off two-step sign-in?" onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <p className={styles.muted} style={{ fontSize: 14 }}>
          Anyone with your password could then sign in without a code. Your account decides where payments are routed, so we recommend keeping it on.
        </p>
        {hasPassword && (
          <Field label="Confirm your password">
            <div style={{ position: "relative" }}><PasswordInput id="pw" value={password} onChange={setPassword} /></div>
          </Field>
        )}
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>Keep it on</button>
          <button className="btn" style={{ background: "#b42318", color: "#fff" }} disabled={busy}>{busy ? "Turning off..." : "Turn off"}</button>
        </div>
      </form>
    </Modal>
  );
}

function SessionsList() {
  const [sessions, setSessions] = useState(null);
  const [error, setError] = useState(null);
  const load = useCallback(() => api.account.sessions().then((r) => setSessions(r.data)).catch(setError), []);
  useEffect(() => { load(); }, [load]);

  async function revoke(id) {
    try { await api.account.revokeSession(id); load(); } catch (e) { setError(e); }
  }
  async function revokeOthers() {
    try { await api.account.revokeOtherSessions(); load(); } catch (e) { setError(e); }
  }

  const others = sessions?.filter((s) => !s.current).length || 0;
  return (
    <div style={{ marginTop: 16 }}>
      <div className={styles.rowBetween}>
        <div style={{ fontWeight: 700, display: "flex", gap: 6, alignItems: "center" }}><Monitor size={16} /> Active sessions</div>
        {others > 0 && (
          <button className="btn btn-ghost" style={{ padding: "6px 10px", fontSize: 13 }} onClick={revokeOthers}>
            <LogOut size={14} /> Sign out {others} other{others > 1 ? "s" : ""}
          </button>
        )}
      </div>
      <ErrorText error={error} />
      {!sessions && <div className={styles.skeletonLine} />}
      {sessions?.map((s) => (
        <div key={s.id} className={styles.kv} style={{ alignItems: "center" }}>
          <span style={{ display: "grid", gap: 2, textAlign: "left" }}>
            <span style={{ fontWeight: 600 }}>
              {s.device} {s.current && <span className={styles.badgeSuccess}>this device</span>}
            </span>
            <span className={styles.muted} style={{ fontSize: 12 }}>
              {s.ip} · signed in {fmtDate(s.created_at)} · last active {fmtDate(s.last_seen_at)}
            </span>
          </span>
          {!s.current && <button className="btn btn-ghost" style={{ padding: "6px 10px", fontSize: 13 }} onClick={() => revoke(s.id)}>Sign out</button>}
        </div>
      ))}
    </div>
  );
}

const EVENT_LABELS = {
  signup: "Account created",
  login: "Signed in",
  login_failed: "Failed sign-in attempt",
  login_code_sent: "Sign-in code sent",
  login_code_failed: "Incorrect sign-in code",
  logout: "Signed out",
  session_revoked: "Session signed out",
  refresh_token_reuse: "Suspicious session reuse blocked",
  email_verified: "Email verified",
  password_reset: "Password reset",
  password_changed: "Password changed",
  two_step_enabled: "Two-step sign-in turned on",
  two_step_disabled: "Two-step sign-in turned off",
  google_linked: "Google linked",
  google_unlinked: "Google unlinked",
};
const WARN_EVENTS = new Set(["login_failed", "login_code_failed", "refresh_token_reuse", "two_step_disabled"]);

function ActivityList() {
  const [events, setEvents] = useState(null);
  useEffect(() => { api.account.securityEvents(15).then((r) => setEvents(r.data)).catch(() => setEvents([])); }, []);
  return (
    <div style={{ marginTop: 16 }}>
      <div style={{ fontWeight: 700, display: "flex", gap: 6, alignItems: "center" }}><KeyRound size={16} /> Recent security activity</div>
      {!events && <div className={styles.skeletonLine} />}
      {events?.length === 0 && <div className={styles.empty}>No activity yet.</div>}
      {events?.map((e) => (
        <div key={e.id} className={styles.kv}>
          <span style={{ color: WARN_EVENTS.has(e.type) ? "#b42318" : undefined }}>{EVENT_LABELS[e.type] || e.type}{e.data?.method ? ` (${e.data.method})` : ""}</span>
          <span className={styles.muted} style={{ fontSize: 12 }}>{e.device} · {e.ip} · {fmtDate(e.created_at)}</span>
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------- notifications

const PREVIEWS = [
  { value: "payment_receipt", label: "Payment receipt" },
  { value: "first_payment", label: "First payment" },
  { value: "webhook_failing", label: "Webhook failing" },
];

function NotificationsCard() {
  const [settings, setSettings] = useState(null);
  const [receiptEmail, setReceiptEmail] = useState("");
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [template, setTemplate] = useState("payment_receipt");
  const [preview, setPreview] = useState(null);
  const [test, setTest] = useState("");

  useEffect(() => {
    api.notifications.get().then((s) => { setSettings(s); setReceiptEmail(s.receipt_email); }).catch(setError);
  }, []);
  useEffect(() => {
    setPreview(null);
    api.notifications.preview(template).then(setPreview).catch(setError);
  }, [template]);

  async function update(patch) {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const s = await api.notifications.update(patch);
      setSettings(s);
      setReceiptEmail(s.receipt_email);
      setSaved(true);
    } catch (e) {
      setError(e);
    } finally {
      setSaving(false);
    }
  }

  async function sendTest() {
    setTest("sending");
    try { setTest(`Sent to ${(await api.notifications.testReceipt()).sent_to}`); } catch (e) { setTest(e.message); }
  }

  return (
    <section className={styles.card}>
      <CardTitle icon={Bell} title="Notifications" subtitle="Emails Paytrack sends about your payments" />
      {!settings ? <div className={styles.skeletonLine} /> : (
        <>
          <SettingRow title="Payment receipts" description="Email a receipt for every successful payment you receive.">
            <Toggle label="Payment receipts" checked={settings.payment_receipts} disabled={saving} onChange={(v) => update({ payment_receipts: v })} />
          </SettingRow>
          <form
            className={styles.kv}
            style={{ alignItems: "center", padding: "12px 0", flexWrap: "wrap" }}
            onSubmit={(e) => { e.preventDefault(); update({ receipt_email: receiptEmail }); }}
          >
            <span style={{ display: "grid", gap: 2, flex: "1 1 220px" }}>
              <span style={{ fontWeight: 600 }}>Send receipts and alerts to</span>
              <span className={styles.muted} style={{ fontSize: 13 }}>Leave empty to use your account email. Currently: {settings.receipt_recipient}</span>
            </span>
            <span style={{ display: "flex", gap: 8, flex: "1 1 260px" }}>
              <input className="input" type="email" placeholder="accounts@yourbusiness.co.ke" value={receiptEmail} onChange={(e) => setReceiptEmail(e.target.value)} />
              <button className="btn" style={{ border: "1px solid #e5e7ef" }} disabled={saving || receiptEmail === settings.receipt_email}>Save</button>
            </span>
          </form>
          <SettingRow title="Webhook failure alerts" description="Email when an event can't be delivered to your backend after every retry.">
            <Toggle label="Webhook failure alerts" checked={settings.webhook_failure_alerts} disabled={saving} onChange={(v) => update({ webhook_failure_alerts: v })} />
          </SettingRow>
          <ErrorText error={error} />
          {saved && <div className={styles.muted} style={{ fontSize: 13, marginTop: 6 }}>Saved.</div>}
        </>
      )}

      <div style={{ marginTop: 16 }}>
        <div className={styles.rowBetween} style={{ flexWrap: "wrap" }}>
          <div style={{ fontWeight: 700 }}>Email preview</div>
          <div style={{ display: "flex", gap: 8 }}>
            <select className={styles.select} style={{ width: "auto" }} value={template} onChange={(e) => setTemplate(e.target.value)}>
              {PREVIEWS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
            </select>
            <button className="btn" style={{ border: "1px solid #e5e7ef", whiteSpace: "nowrap" }} disabled={test === "sending"} onClick={sendTest}>
              <Send size={14} /> Send test receipt
            </button>
          </div>
        </div>
        {test && test !== "sending" && <div className={styles.muted} style={{ fontSize: 13, marginTop: 6 }}>{test}</div>}
        <div style={{ marginTop: 10, border: "1px solid #e9ebf4", borderRadius: 12, overflow: "hidden" }}>
          <div style={{ padding: "8px 12px", background: "#f9faff", fontSize: 13, borderBottom: "1px solid #eef1f7" }}>
            <span className={styles.muted}>Subject: </span>{preview?.subject || "…"}
          </div>
          {preview ? (
            <iframe title="Email preview" srcDoc={preview.html} sandbox="" style={{ width: "100%", height: 720, border: 0, display: "block", background: "#f6f7ff" }} />
          ) : <div className={styles.skeletonLine} style={{ margin: 12 }} />}
        </div>
      </div>
    </section>
  );
}
