"use client";

import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api, getBaseUrl } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { CopyButton, ErrorText, Field, Modal, StatusBadge } from "@/components/ui.jsx";

export default function ApiKeysPage() {
  const [keys, setKeys] = useState(null);
  const [error, setError] = useState(null);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState(null);
  const [revoking, setRevoking] = useState(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    setError(null);
    try {
      setKeys((await api.apiKeys.list()).data);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, []);

  async function revoke(k) {
    setBusy(true);
    try {
      await api.apiKeys.revoke(k.id);
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
      setRevoking(null);
    }
  }

  return (
    <>
      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <div className={styles.h2}>API keys</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>
              Use a key from your backend as <code>Authorization: Bearer ptk_…</code>. Keep keys server-side.
            </div>
          </div>
          <button className="btn btn-primary" onClick={() => setCreating(true)}><Plus size={16} /> Create key</button>
        </div>

        <div className={styles.table} style={{ "--cols": "1.4fr 1.2fr 1fr 1.2fr 1.2fr 0.8fr" }}>
          <div className={styles.thead}>
            <div>Name</div><div>Key</div><div>Status</div><div>Created</div><div>Last used</div><div />
          </div>
          <div className={styles.tbody}>
            {!keys && !error && <div className={styles.empty}>Loading...</div>}
            {keys?.length === 0 && <div className={styles.empty}>No API keys yet.</div>}
            {keys?.map((k) => (
              <div key={k.id} className={styles.trow} style={{ opacity: k.revoked_at ? 0.6 : 1 }}>
                <div className={styles.cellTitle}>{k.name}</div>
                <div className={styles.mono} style={{ letterSpacing: 0, fontSize: 13 }}>{k.prefix}…</div>
                <div>
                  {k.revoked_at
                    ? <StatusBadge status="revoked" tone="danger" />
                    : <StatusBadge status="active" tone="success" />}
                </div>
                <div className={styles.cellSub}>{fmtDate(k.created_at)}</div>
                <div className={styles.cellSub}>{k.last_used_at ? fmtDate(k.last_used_at) : "Never"}</div>
                <div style={{ textAlign: "right" }}>
                  {!k.revoked_at && (
                    <button className="btn btn-ghost" style={{ padding: "6px 10px", color: "#b42318" }} onClick={() => setRevoking(k)}>Revoke</button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
        <ErrorText error={error} />
      </section>

      <section className={styles.card}>
        <div className={styles.rowBetween} style={{ marginBottom: 8 }}>
          <div style={{ fontWeight: 700 }}>Quick start</div>
          <a href="/docs#quickstart" target="_blank" className="btn btn-ghost" style={{ padding: "6px 10px" }}>Full API docs →</a>
        </div>
        <pre className={styles.codeBox}>{`curl -X POST ${getBaseUrl()}/v1/payments \\
  -H "Authorization: Bearer $PAYTRACK_API_KEY" \\
  -H "Idempotency-Key: order-123" \\
  -H "Content-Type: application/json" \\
  -d '{"channel_id":"chn_…","amount":10,"phone":"0712345678"}'`}</pre>
      </section>

      {creating && (
        <CreateKeyModal
          onClose={() => setCreating(false)}
          onCreated={(k) => {
            setCreating(false);
            setCreated(k);
            setKeys((prev) => [k, ...(prev || [])]);
          }}
        />
      )}

      {created && (
        <Modal title="Save your API key" onClose={() => setCreated(null)} footer={<button className="btn btn-primary" onClick={() => setCreated(null)}>I&apos;ve saved it</button>}>
          <div className={styles.notice} style={{ marginBottom: 12 }}>
            This is the only time the full key is shown. Store it somewhere safe, like your backend&apos;s secret manager.
          </div>
          <div className={styles.secretRow}>
            <span className={styles.secretValue}>{created.key}</span>
            <CopyButton text={created.key} />
          </div>
        </Modal>
      )}

      {revoking && (
        <Modal
          title={`Revoke "${revoking.name}"?`}
          onClose={() => setRevoking(null)}
          footer={
            <>
              <button className="btn btn-ghost" onClick={() => setRevoking(null)} disabled={busy}>Cancel</button>
              <button className="btn btn-primary" style={{ background: "#b42318", boxShadow: "none" }} onClick={() => revoke(revoking)} disabled={busy}>
                {busy ? "Revoking..." : "Revoke key"}
              </button>
            </>
          }
        >
          <div className={styles.muted}>Requests using <span className={styles.mono} style={{ letterSpacing: 0 }}>{revoking.prefix}…</span> will be rejected immediately. This can&apos;t be undone.</div>
        </Modal>
      )}
    </>
  );
}

function CreateKeyModal({ onClose, onCreated }) {
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  async function submit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      onCreated(await api.apiKeys.create(name.trim()));
    } catch (err) {
      setError(err);
      setSubmitting(false);
    }
  }

  return (
    <Modal title="Create API key" onClose={onClose}>
      <form onSubmit={submit}>
        <Field label="Name" hint="Something that tells you where the key is used, e.g. “production backend”.">
          <input className="input" autoFocus value={name} onChange={(e) => setName(e.target.value)} required />
        </Field>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>{submitting ? "Creating..." : "Create key"}</button>
        </div>
      </form>
    </Modal>
  );
}
