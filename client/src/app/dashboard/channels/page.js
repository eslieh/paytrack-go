"use client";

import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import styles from "../styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, Field, Modal, StatusBadge } from "@/components/ui.jsx";

const EMPTY = {
  name: "",
  type: "paybill",
  environment: "sandbox",
  short_code: "",
  till_number: "",
  consumer_key: "",
  consumer_secret: "",
  passkey: "",
};

export default function ChannelsPage() {
  const [channels, setChannels] = useState(null);
  const [error, setError] = useState(null);
  const [creating, setCreating] = useState(false);
  const [deactivating, setDeactivating] = useState(null);

  async function load() {
    setError(null);
    try {
      setChannels((await api.channels.list()).data);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => { load(); }, []);

  async function deactivate(c) {
    try {
      await api.channels.deactivate(c.id);
      setChannels((prev) => prev.map((x) => (x.id === c.id ? { ...x, active: false } : x)));
      setDeactivating(null);
    } catch (e) {
      setError(e);
      setDeactivating(null);
    }
  }

  return (
    <>
      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <div className={styles.h2}>Channels</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>Your Daraja PayBill and Till credentials. Payments are sent through a channel.</div>
          </div>
          <button className="btn btn-primary" onClick={() => setCreating(true)}><Plus size={16} /> Add channel</button>
        </div>

        <div className={styles.table} style={{ "--cols": "1.6fr 1fr 1fr 1fr 1.2fr 0.8fr" }}>
          <div className={styles.thead}>
            <div>Name</div><div>Type</div><div>Number</div><div>Status</div><div>Created</div><div />
          </div>
          <div className={styles.tbody}>
            {!channels && !error && <div className={styles.empty}>Loading...</div>}
            {channels?.length === 0 && <div className={styles.empty}>No channels yet. Add your Daraja credentials to start charging.</div>}
            {channels?.map((c) => (
              <div key={c.id} className={styles.trow}>
                <div>
                  <div className={styles.cellTitle}>{c.name}</div>
                  <div className={styles.cellSub}>{c.id}</div>
                </div>
                <div style={{ textTransform: "capitalize" }}>{c.type}</div>
                <div>
                  {c.type === "till" ? c.till_number : c.short_code}
                  {c.type === "till" && <div className={styles.cellSub}>Store {c.short_code}</div>}
                </div>
                <div>
                  {c.active
                    ? <StatusBadge status={c.environment} tone={c.environment === "production" ? "success" : "warning"} />
                    : <StatusBadge status="inactive" tone="neutral" />}
                </div>
                <div className={styles.cellSub}>{fmtDate(c.created_at)}</div>
                <div style={{ textAlign: "right" }}>
                  {c.active && (
                    <button className="btn btn-ghost" style={{ padding: "6px 10px", color: "#b42318" }} onClick={() => setDeactivating(c)}>
                      Deactivate
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
        <ErrorText error={error} />
      </section>

      {creating && (
        <NewChannelModal
          onClose={() => setCreating(false)}
          onCreated={(c) => { setChannels((prev) => [c, ...(prev || [])]); setCreating(false); }}
        />
      )}

      {deactivating && (
        <Modal
          title={`Deactivate ${deactivating.name}?`}
          onClose={() => setDeactivating(null)}
          footer={
            <>
              <button className="btn btn-ghost" onClick={() => setDeactivating(null)}>Cancel</button>
              <button className="btn btn-primary" style={{ background: "#b42318", boxShadow: "none" }} onClick={() => deactivate(deactivating)}>Deactivate</button>
            </>
          }
        >
          <div className={styles.muted}>New payments on this channel will be rejected. Payments already in progress still complete. This can't be undone; add the channel again to reuse it.</div>
        </Modal>
      )}
    </>
  );
}

function NewChannelModal({ onClose, onCreated }) {
  const [form, setForm] = useState(EMPTY);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });
  const isTill = form.type === "till";

  async function submit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const body = { ...form };
      if (!isTill) delete body.till_number;
      for (const k of Object.keys(body)) body[k] = body[k].trim();
      onCreated(await api.channels.create(body));
    } catch (err) {
      setError(err);
      setSubmitting(false);
    }
  }

  return (
    <Modal title="Add Daraja channel" onClose={onClose}>
      <form onSubmit={submit}>
        <div className={styles.notice}>
          Credentials are encrypted at rest and never shown again. Get them from your app on the Safaricom Daraja portal.
        </div>
        <div className={styles.formGrid}>
          <Field label="Name">
            <input className="input" placeholder="Main shop" value={form.name} onChange={set("name")} required />
          </Field>
          <Field label="Environment">
            <select className={styles.select} value={form.environment} onChange={set("environment")}>
              <option value="sandbox">Sandbox</option>
              <option value="production">Production</option>
            </select>
          </Field>
          <Field label="Type">
            <select className={styles.select} value={form.type} onChange={set("type")}>
              <option value="paybill">PayBill</option>
              <option value="till">Till (Buy Goods)</option>
            </select>
          </Field>
          <Field label={isTill ? "Store number" : "PayBill number"} hint="5–7 digits">
            <input className="input" inputMode="numeric" pattern="\d{5,7}" placeholder={isTill ? "Head office / store no." : "174379"} value={form.short_code} onChange={set("short_code")} required />
          </Field>
          {isTill && (
            <Field label="Till number" hint="5–7 digits">
              <input className="input" inputMode="numeric" pattern="\d{5,7}" value={form.till_number} onChange={set("till_number")} required />
            </Field>
          )}
          <Field label="Consumer key">
            <input className="input" autoComplete="off" value={form.consumer_key} onChange={set("consumer_key")} required />
          </Field>
          <Field label="Consumer secret">
            <input className="input" type="password" autoComplete="new-password" value={form.consumer_secret} onChange={set("consumer_secret")} required />
          </Field>
          <Field label="Passkey">
            <input className="input" type="password" autoComplete="new-password" value={form.passkey} onChange={set("passkey")} required />
          </Field>
        </div>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>{submitting ? "Saving..." : "Add channel"}</button>
        </div>
      </form>
    </Modal>
  );
}
