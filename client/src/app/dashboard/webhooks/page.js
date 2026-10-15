"use client";

import { useCallback, useEffect, useState } from "react";
import { Eye, EyeOff, RefreshCw } from "lucide-react";
import styles from "../styles.module.css";
import { api } from "@/lib/api";
import { fmtDate, maskSecret } from "@/lib/format";
import { CopyButton, ErrorText, Field, Modal, StatusBadge } from "@/components/ui.jsx";

const DELIVERY_STATUSES = ["pending", "delivering", "delivered", "failed"];

const VERIFY_SNIPPET = `// Node.js (Express) — verify a PayTrack webhook
const crypto = require("crypto");

app.post("/hooks/paytrack", express.raw({ type: "application/json" }), (req, res) => {
  const header = req.get("PayTrack-Signature") || "";      // t=<unix>,v1=<hex>
  const parts = Object.fromEntries(header.split(",").map((p) => p.split("=")));
  const expected = crypto
    .createHmac("sha256", process.env.PAYTRACK_SIGNING_SECRET)
    .update(\`\${parts.t}.\${req.body}\`)
    .digest("hex");

  const fresh = Math.abs(Date.now() / 1000 - Number(parts.t)) < 300;
  const valid = parts.v1 && crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(parts.v1));
  if (!fresh || !valid) return res.sendStatus(400);

  const event = JSON.parse(req.body);                       // dedupe on event.id
  // event.type: payment.succeeded | payment.failed | payment.cancelled | payment.expired
  res.sendStatus(200);
});`;

export default function WebhooksPage() {
  return (
    <>
      <EndpointCard />
      <DeliveriesCard />
    </>
  );
}

function EndpointCard() {
  const [webhook, setWebhook] = useState(null);
  const [url, setUrl] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [reveal, setReveal] = useState(false);
  const [confirmRotate, setConfirmRotate] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    api.webhook.get().then((w) => { setWebhook(w); setUrl(w.url || ""); }).catch(setError);
  }, []);

  async function save(e) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const w = await api.webhook.setUrl(url.trim());
      setWebhook(w);
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
    } catch (err) {
      setError(err);
    } finally {
      setSaving(false);
    }
  }

  async function rotate() {
    setRotating(true);
    setError(null);
    try {
      setWebhook(await api.webhook.rotateSecret());
      setReveal(true);
    } catch (err) {
      setError(err);
    } finally {
      setRotating(false);
      setConfirmRotate(false);
    }
  }

  return (
    <section className={styles.card}>
      <div className={styles.h2}>Webhook endpoint</div>
      <div className={styles.muted} style={{ fontSize: 13 }}>
        PayTrack POSTs a signed event here when a payment succeeds, fails, is cancelled or expires.
      </div>

      <form onSubmit={save} style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
        <input
          className="input"
          type="url"
          placeholder="https://your.app/hooks/paytrack"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          style={{ flex: "1 1 320px" }}
          required
        />
        <button className="btn btn-primary" type="submit" disabled={saving || !webhook || url.trim() === (webhook?.url || "")}>
          {saving ? "Saving..." : saved ? "Saved" : "Save"}
        </button>
      </form>

      <div style={{ marginTop: 16 }}>
        <Field label="Signing secret">
          <div className={styles.secretRow}>
            <span className={styles.secretValue}>{webhook ? (reveal ? webhook.signing_secret : maskSecret(webhook.signing_secret)) : "—"}</span>
            <button type="button" className="btn btn-ghost" style={{ padding: "6px 10px" }} onClick={() => setReveal((v) => !v)} aria-label={reveal ? "Hide secret" : "Show secret"}>
              {reveal ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
            <CopyButton text={webhook?.signing_secret} />
            <button type="button" className="btn btn-ghost" style={{ padding: "6px 10px" }} onClick={() => setConfirmRotate(true)} disabled={!webhook}>
              <RefreshCw size={16} /> Rotate
            </button>
          </div>
        </Field>
      </div>

      <details style={{ marginTop: 16 }}>
        <summary style={{ cursor: "pointer", fontWeight: 600, fontSize: 14 }}>How to verify signatures</summary>
        <div className={styles.muted} style={{ fontSize: 13, margin: "8px 0" }}>
          Each request carries <code>PayTrack-Event-Id</code>, <code>PayTrack-Event-Type</code>, <code>PayTrack-Delivery-Id</code> and{" "}
          <code>PayTrack-Signature: t=&lt;unix&gt;,v1=&lt;hex&gt;</code>, where v1 is HMAC-SHA256 of <code>&quot;&lt;t&gt;.&lt;raw body&gt;&quot;</code> keyed with
          your signing secret. Reject timestamps older than 5 minutes, dedupe on the event ID, and respond 2xx within 10 seconds.
          Non-2xx responses are retried for about 16 hours.
        </div>
        <pre className={styles.codeBox}>{VERIFY_SNIPPET}</pre>
      </details>

      <ErrorText error={error} />

      {confirmRotate && (
        <Modal
          title="Rotate signing secret?"
          onClose={() => setConfirmRotate(false)}
          footer={
            <>
              <button className="btn btn-ghost" onClick={() => setConfirmRotate(false)} disabled={rotating}>Cancel</button>
              <button className="btn btn-primary" onClick={rotate} disabled={rotating}>{rotating ? "Rotating..." : "Rotate secret"}</button>
            </>
          }
        >
          <div className={styles.muted}>
            The current secret stops working immediately. Update your backend with the new secret straight away, or webhook verification will fail.
          </div>
        </Modal>
      )}
    </section>
  );
}

function DeliveriesCard() {
  const [status, setStatus] = useState("");
  const [rows, setRows] = useState([]);
  const [cursor, setCursor] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [redelivering, setRedelivering] = useState(null);
  const [viewing, setViewing] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(async (s) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.webhook.deliveries({ status: s });
      setRows(res.data);
      setCursor(res.next_cursor || null);
      setHasMore(res.has_more);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(status); }, [status, load]);

  async function loadMore() {
    setLoadingMore(true);
    try {
      const res = await api.webhook.deliveries({ status, cursor });
      setRows((prev) => prev.concat(res.data));
      setCursor(res.next_cursor || null);
      setHasMore(res.has_more);
    } catch (e) {
      setError(e);
    } finally {
      setLoadingMore(false);
    }
  }

  async function redeliver(d) {
    setRedelivering(d.id);
    setError(null);
    try {
      const updated = await api.webhook.redeliver(d.id);
      setRows((prev) => prev.map((x) => (x.id === d.id ? updated : x)));
    } catch (e) {
      setError(e);
    } finally {
      setRedelivering(null);
    }
  }

  return (
    <section className={styles.card}>
      <div className={styles.rowBetween}>
        <div className={styles.h2}>Deliveries</div>
        <div style={{ display: "flex", gap: 8 }}>
          <select className={styles.select} value={status} onChange={(e) => setStatus(e.target.value)} style={{ width: "auto" }}>
            <option value="">All</option>
            {DELIVERY_STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <button className="btn btn-ghost" onClick={() => load(status)} disabled={loading} aria-label="Refresh"><RefreshCw size={16} /></button>
        </div>
      </div>

      <div className={styles.table} style={{ "--cols": "1.5fr 1fr 1.4fr 1.2fr 0.9fr" }}>
        <div className={styles.thead}>
          <div>Event</div><div>Status</div><div>Last attempt</div><div>Created</div><div />
        </div>
        <div className={styles.tbody}>
          {loading && <div className={styles.empty}>Loading...</div>}
          {!loading && rows.length === 0 && <div className={styles.empty}>No deliveries yet.</div>}
          {!loading && rows.map((d) => (
            <div key={d.id} className={`${styles.trow} ${styles.clickRow}`} onClick={() => setViewing(d)}>
              <div>
                <div className={styles.cellTitle}>{d.event_type}</div>
                <div className={styles.cellSub}>{d.payment_id}</div>
              </div>
              <div><StatusBadge status={d.status} /></div>
              <div className={styles.cellSub}>
                {d.attempts} attempt{d.attempts === 1 ? "" : "s"}
                {d.last_status_code ? ` · HTTP ${d.last_status_code}` : ""}
                {d.last_error && <div title={d.last_error}>{d.last_error}</div>}
                {d.status !== "delivered" && d.next_attempt_at && <div>Next: {fmtDate(d.next_attempt_at)}</div>}
              </div>
              <div className={styles.cellSub}>{fmtDate(d.created_at)}</div>
              <div style={{ textAlign: "right" }}>
                <button
                  className="btn btn-ghost"
                  style={{ padding: "6px 10px" }}
                  onClick={(e) => { e.stopPropagation(); redeliver(d); }}
                  disabled={redelivering === d.id}
                >
                  {redelivering === d.id ? "Queuing..." : "Redeliver"}
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className={styles.loadMoreRow}>
        {hasMore && <button className="btn btn-ghost" onClick={loadMore} disabled={loadingMore}>{loadingMore ? "Loading..." : "Load more"}</button>}
      </div>
      <ErrorText error={error} />

      {viewing && (
        <Modal title={viewing.event_type} onClose={() => setViewing(null)} footer={<button className="btn btn-primary" onClick={() => setViewing(null)}>Close</button>}>
          <div className={styles.cellSub} style={{ marginBottom: 8 }}>
            Event {viewing.event_id} · Delivery {viewing.id}
            {viewing.delivered_at && <> · Delivered {fmtDate(viewing.delivered_at)}</>}
          </div>
          <pre className={styles.codeBox}>{JSON.stringify(viewing.payload, null, 2)}</pre>
        </Modal>
      )}
    </section>
  );
}
