"use client";

import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import styles from "../styles.module.css";
import { api } from "@/lib/api";
import { CHANNEL_TYPE_LABEL, destinationLabel, fmtDate } from "@/lib/format";
import { ErrorText, Field, Modal, Skeleton, SkeletonRows, StatusBadge } from "@/components/ui.jsx";
import { Busy } from "@/components/Busy.jsx";

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
    } catch (e) {
      setError(e);
    } finally {
      setDeactivating(null);
    }
  }

  return (
    <>
      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <div className={styles.h2}>Channels</div>
            <div className={styles.muted} style={{ fontSize: 13 }}>
              Where your money goes: a bank account, a PayBill or a Till. Every payment is sent to a channel.
            </div>
          </div>
          <button className="btn btn-primary" onClick={() => setCreating(true)}><Plus size={16} /> Add channel</button>
        </div>

        <div className={styles.table} style={{ "--cols": "1.5fr 1fr 1.8fr 0.9fr 1.2fr 0.8fr" }}>
          <div className={styles.thead}>
            <div>Name</div><div>Type</div><div>Destination</div><div>Status</div><div>Created</div><div />
          </div>
          <div className={styles.tbody}>
            {!channels && !error && <SkeletonRows rows={3} cols={5} />}
            {channels?.length === 0 && <div className={styles.empty}>No channels yet. Add a bank account, PayBill or Till to start collecting.</div>}
            {channels?.map((c) => (
              <div key={c.id} className={styles.trow}>
                <div>
                  <div className={styles.cellTitle}>{c.name}</div>
                  <div className={styles.cellSub}>{c.id}</div>
                </div>
                <div>{CHANNEL_TYPE_LABEL[c.type] || c.type}</div>
                <div className={styles.cellTitle} style={{ fontWeight: 400 }} title={destinationLabel(c)}>{destinationLabel(c)}</div>
                <div>
                  {c.active ? <StatusBadge status="active" tone="success" /> : <StatusBadge status="inactive" tone="neutral" />}
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
          <div className={styles.muted}>New payments on this channel will be rejected. Payments already in progress still complete. This can&apos;t be undone; add the channel again to reuse it.</div>
        </Modal>
      )}
    </>
  );
}

function NewChannelModal({ onClose, onCreated }) {
  const [form, setForm] = useState({ name: "", type: "bank", bank_code: "", business_number: "", account_number: "" });
  const [banks, setBanks] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  useEffect(() => {
    api.banks.list().then((r) => setBanks(r.data)).catch((e) => { setBanks([]); setError(e); });
  }, []);

  async function submit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    const body = { name: form.name.trim(), type: form.type };
    if (form.type === "bank") {
      body.bank_code = form.bank_code;
      body.account_number = form.account_number.trim();
    } else {
      body.business_number = form.business_number.trim();
      if (form.type === "paybill" && form.account_number.trim()) body.account_number = form.account_number.trim();
    }
    try {
      onCreated(await api.channels.create(body));
    } catch (err) {
      setError(err);
      setSubmitting(false);
    }
  }

  return (
    <Modal title="Add channel" onClose={onClose}>
      <form onSubmit={submit}>
        <div className={styles.notice}>
          PayTrack sends the M-Pesa prompt and routes the money straight to this destination. You don&apos;t need your own Daraja app.
        </div>
        <div className={styles.formGrid}>
          <Field label="Name">
            <input className="input" placeholder="Main account" value={form.name} onChange={set("name")} required />
          </Field>
          <Field label="Type">
            <select className={styles.select} value={form.type} onChange={set("type")}>
              {Object.entries(CHANNEL_TYPE_LABEL).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </Field>

          {form.type === "bank" && (
            <>
              <Field label="Bank">
                {banks ? (
                  <select className={styles.select} value={form.bank_code} onChange={set("bank_code")} required>
                    <option value="" disabled>Choose a bank</option>
                    {banks.map((b) => <option key={b.code} value={b.code}>{b.name}</option>)}
                  </select>
                ) : <Skeleton h={44} r={10} />}
              </Field>
              <Field label="Account number" hint="The bank account credited, up to 20 characters">
                <input className="input" maxLength={20} value={form.account_number} onChange={set("account_number")} required />
              </Field>
            </>
          )}

          {form.type !== "bank" && (
            <Field label={form.type === "till" ? "Till number" : "PayBill number"} hint="5–7 digits">
              <input className="input" inputMode="numeric" pattern="\d{5,7}" value={form.business_number} onChange={set("business_number")} required />
            </Field>
          )}

          {form.type === "paybill" && (
            <Field label="Default account number" hint="Optional. Used when a payment doesn't set its own account reference.">
              <input className="input" maxLength={20} value={form.account_number} onChange={set("account_number")} />
            </Field>
          )}
        </div>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>{submitting ? <Busy>Saving…</Busy> : "Add channel"}</button>
        </div>
      </form>
    </Modal>
  );
}
