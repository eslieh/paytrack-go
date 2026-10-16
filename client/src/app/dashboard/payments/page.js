"use client";

import Link from "next/link";
import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Plus, RefreshCw } from "lucide-react";
import styles from "../styles.module.css";
import { api } from "@/lib/api";
import { destinationLabel, fmtDate, fmtKES, maskPhone, newIdempotencyKey } from "@/lib/format";
import { ErrorText, Field, KV, Modal, PageSkeleton, SkeletonKV, SkeletonRows, StatusBadge, useInView } from "@/components/ui.jsx";
import { Busy } from "@/components/Busy.jsx";

const STATUSES = ["created", "pending", "succeeded", "failed", "cancelled", "expired"];
const OPEN = new Set(["created", "pending"]);
const COLS = { "--cols": "1.6fr 1fr 1fr 1fr 1.3fr" };

export default function PaymentsPage() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <Payments />
    </Suspense>
  );
}

function Payments() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const selectedId = params.get("id");

  const [filters, setFilters] = useState({ status: "", channel_id: "", phone: "" });
  const [rows, setRows] = useState([]);
  const [cursor, setCursor] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const sentinel = useRef(null);
  const [error, setError] = useState(null);
  const [channels, setChannels] = useState([]);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    api.channels.list().then((r) => setChannels(r.data)).catch(() => {});
  }, []);

  const load = useCallback(async (f) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.payments.list(f);
      setRows(res.data);
      setCursor(res.next_cursor || null);
      setHasMore(res.has_more);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }, []);

  // Debounce so typing a phone number doesn't fire a request per keystroke.
  useEffect(() => {
    const t = setTimeout(() => load(filters), 300);
    return () => clearTimeout(t);
  }, [filters, load]);

  async function loadMore() {
    if (!hasMore || loadingMore) return;
    setLoadingMore(true);
    try {
      const res = await api.payments.list({ ...filters, cursor });
      setRows((prev) => prev.concat(res.data));
      setCursor(res.next_cursor || null);
      setHasMore(res.has_more);
    } catch (e) {
      setError(e);
    } finally {
      setLoadingMore(false);
    }
  }

  // Lazy-load the next page as the list's end scrolls into view.
  useInView(sentinel, loadMore, { enabled: hasMore && !loading && !loadingMore });

  const select = (id) => router.replace(id ? `${pathname}?id=${encodeURIComponent(id)}` : pathname, { scroll: false });
  const channelName = (id) => channels.find((c) => c.id === id)?.name || id;

  // Keep a row in the list in sync with the detail modal's latest view.
  const onPaymentUpdate = useCallback((p) => setRows((prev) => prev.map((r) => (r.id === p.id ? p : r))), []);

  return (
    <>
      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div className={styles.h2}>Payments</div>
          <div style={{ display: "flex", gap: 8 }}>
            <button className="btn btn-ghost" onClick={() => load(filters)} disabled={loading} aria-label="Refresh">
              <RefreshCw size={16} />
            </button>
            <button className="btn btn-primary" onClick={() => setCreating(true)}>
              <Plus size={16} /> New STK push
            </button>
          </div>
        </div>

        <div className={styles.filters}>
          <select className={styles.select} value={filters.status} onChange={(e) => setFilters({ ...filters, status: e.target.value })}>
            <option value="">All statuses</option>
            {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <select className={styles.select} value={filters.channel_id} onChange={(e) => setFilters({ ...filters, channel_id: e.target.value })}>
            <option value="">All channels</option>
            {channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
          <input className="input" placeholder="Phone (2547…)" value={filters.phone} onChange={(e) => setFilters({ ...filters, phone: e.target.value.trim() })} />
        </div>

        <div className={styles.table} style={COLS}>
          <div className={styles.thead}>
            <div>Reference</div><div>Amount</div><div>Status</div><div>Channel</div><div>Date</div>
          </div>
          <div className={styles.tbody}>
            {loading && <SkeletonRows rows={6} cols={5} />}
            {!loading && rows.length === 0 && <div className={styles.empty}>No payments match.</div>}
            {!loading && rows.map((p) => (
              <div key={p.id} className={`${styles.trow} ${styles.clickRow}`} onClick={() => select(p.id)}>
                <div>
                  <div className={styles.cellTitle}>{p.account_reference}</div>
                  <div className={styles.cellSub}>{maskPhone(p.phone)} · {p.description}</div>
                </div>
                <div style={{ fontWeight: 600 }}>{fmtKES(p.amount)}</div>
                <div><StatusBadge status={p.status} /></div>
                <div>
                  <div className={styles.cellSub} style={{ color: "#0f1222" }}>{channelName(p.channel_id)}</div>
                  <div className={styles.cellSub}>{destinationLabel(p)}</div>
                </div>
                <div className={styles.cellSub}>{fmtDate(p.created_at)}</div>
              </div>
            ))}
            {loadingMore && <SkeletonRows rows={3} cols={5} />}
          </div>
        </div>
        <div className={styles.loadMoreRow} ref={sentinel}>
          {hasMore ? (
            <button className="btn btn-ghost" onClick={loadMore} disabled={loadingMore}>{loadingMore ? <Busy>Loading…</Busy> : "Load more"}</button>
          ) : (
            !loading && rows.length > 0 && <span className={styles.muted}>No more payments</span>
          )}
        </div>
        <ErrorText error={error} />
      </section>

      {selectedId && (
        <PaymentDetail id={selectedId} channelName={channelName} onClose={() => select(null)} onUpdate={onPaymentUpdate} />
      )}
      {creating && (
        <NewPaymentModal
          channels={channels.filter((c) => c.active)}
          onClose={() => setCreating(false)}
          onCreated={(p) => {
            setCreating(false);
            setRows((prev) => [p, ...prev.filter((r) => r.id !== p.id)]);
            select(p.id);
          }}
        />
      )}
    </>
  );
}

function PaymentDetail({ id, channelName, onClose, onUpdate }) {
  const [p, setP] = useState(null);
  const [deliveries, setDeliveries] = useState(null);
  const [error, setError] = useState(null);
  const [redelivering, setRedelivering] = useState(null);

  const loadDeliveries = useCallback(() => {
    api.webhook.deliveries({ payment_id: id, limit: 20 }).then((r) => setDeliveries(r.data)).catch(() => setDeliveries([]));
  }, [id]);

  useEffect(() => {
    let mounted = true;
    let timer;
    async function tick() {
      try {
        const res = await api.payments.get(id);
        if (!mounted) return;
        setP(res);
        onUpdate(res);
        // Poll while the customer is still answering the STK prompt.
        if (OPEN.has(res.status)) timer = setTimeout(tick, 3000);
        else loadDeliveries();
      } catch (e) {
        if (mounted) setError(e);
      }
    }
    tick();
    return () => { mounted = false; clearTimeout(timer); };
  }, [id, onUpdate, loadDeliveries]);

  async function redeliver(d) {
    setRedelivering(d.id);
    try {
      const updated = await api.webhook.redeliver(d.id);
      setDeliveries((prev) => prev.map((x) => (x.id === d.id ? updated : x)));
    } catch (e) {
      setError(e);
    } finally {
      setRedelivering(null);
    }
  }

  return (
    <Modal
      title={p ? `${fmtKES(p.amount)} · ${p.account_reference}` : "Payment"}
      onClose={onClose}
      footer={
        <>
          {p?.status === "succeeded" && (
            <Link className="btn btn-ghost" href={`/receipt?id=${encodeURIComponent(p.id)}`} target="_blank">Receipt</Link>
          )}
          <button className="btn btn-primary" onClick={onClose}>Close</button>
        </>
      }
    >
      {!p && !error && <SkeletonKV rows={7} />}
      {p && (
        <>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            <StatusBadge status={p.status} />
            {OPEN.has(p.status) && <span className={styles.cellSub}>Waiting for the customer to respond…</span>}
          </div>
          <KV label="Payment ID"><span className={styles.mono} style={{ letterSpacing: 0 }}>{p.id}</span></KV>
          <KV label="Phone">{p.phone}</KV>
          <KV label="Description">{p.description}</KV>
          <KV label="Channel">{channelName(p.channel_id)}</KV>
          <KV label="Paid to">{destinationLabel(p)}</KV>
          {p.destination_type !== "bank" && <KV label="Account reference">{p.account_reference}</KV>}
          {p.mpesa_receipt_number && <KV label="M-Pesa receipt"><strong>{p.mpesa_receipt_number}</strong></KV>}
          {p.result_desc && <KV label="Result">{p.result_code != null ? `${p.result_code} · ` : ""}{p.result_desc}</KV>}
          {p.failure_reason && <KV label="Failure reason">{p.failure_reason}</KV>}
          <KV label="Created">{fmtDate(p.created_at)}</KV>
          <KV label="Completed">{fmtDate(p.completed_at)}</KV>
          {p.checkout_request_id && <KV label="Checkout request">{p.checkout_request_id}</KV>}
          {p.metadata && Object.keys(p.metadata).length > 0 && (
            Object.entries(p.metadata).map(([k, v]) => <KV key={k} label={`metadata.${k}`}>{v}</KV>)
          )}

          {deliveries && deliveries.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <div style={{ fontWeight: 700, marginBottom: 4 }}>Webhook deliveries</div>
              {deliveries.map((d) => (
                <div key={d.id} className={styles.kv} style={{ alignItems: "center" }}>
                  <span>
                    {d.event_type} <StatusBadge status={d.status} />
                    <div className={styles.cellSub}>
                      {d.attempts} attempt{d.attempts === 1 ? "" : "s"}
                      {d.last_status_code ? ` · HTTP ${d.last_status_code}` : ""}
                      {d.last_error ? ` · ${d.last_error}` : ""}
                    </div>
                  </span>
                  <button className="btn btn-ghost" style={{ padding: "6px 10px" }} onClick={() => redeliver(d)} disabled={redelivering === d.id}>
                    {redelivering === d.id ? <Busy>Queuing…</Busy> : "Redeliver"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </>
      )}
      <ErrorText error={error} />
    </Modal>
  );
}

function NewPaymentModal({ channels, onClose, onCreated }) {
  const [form, setForm] = useState({
    channel_id: channels[0]?.id || "",
    amount: "",
    phone: "",
    account_reference: "",
    description: "",
  });
  // One key per form: resubmitting after a network error replays instead of double-charging.
  const [idemKey] = useState(newIdempotencyKey);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });
  const channel = channels.find((c) => c.id === form.channel_id);
  // bank: always the channel's account (must be omitted); paybill: overrides the channel default; till: optional.
  const refMode = channel?.type === "bank" ? "fixed" : channel?.type === "paybill" && !channel.account_number ? "required" : "optional";

  async function submit(e) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const ref = form.account_reference.trim();
      const body = {
        channel_id: form.channel_id,
        amount: Number(form.amount),
        phone: form.phone.trim(),
        ...(refMode !== "fixed" && ref ? { account_reference: ref } : {}),
        ...(form.description.trim() ? { description: form.description.trim() } : {}),
      };
      onCreated(await api.payments.create(body, idemKey));
    } catch (err) {
      setError(err);
      setSubmitting(false);
    }
  }

  if (channels.length === 0) {
    return (
      <Modal title="New STK push" onClose={onClose} footer={<Link className="btn btn-primary" href="/dashboard/channels">Add a channel</Link>}>
        <div className={styles.muted}>You need an active channel (bank account, PayBill or Till) before you can charge customers.</div>
      </Modal>
    );
  }

  return (
    <Modal title="New STK push" onClose={onClose}>
      <form onSubmit={submit}>
        <div className={styles.notice}>
          The customer gets an M-Pesa prompt on their phone. The final status arrives within about a minute.
        </div>
        <div className={styles.formGrid}>
          <Field label="Channel">
            <select className={styles.select} value={form.channel_id} onChange={set("channel_id")} required>
              {channels.map((c) => <option key={c.id} value={c.id}>{c.name} · {destinationLabel(c)}</option>)}
            </select>
          </Field>
          <Field label="Amount (KES)" hint="Whole shillings, 1 – 250,000">
            <input className="input" type="number" min={1} max={250000} step={1} value={form.amount} onChange={set("amount")} required />
          </Field>
          <Field label="Phone" hint="07…, 01… or 254…">
            <input className="input" inputMode="tel" placeholder="0712345678" value={form.phone} onChange={set("phone")} required />
          </Field>
          {refMode === "fixed" ? (
            <Field label="Account reference" hint="Bank channels always credit the channel's account">
              <input className="input" value={channel.account_number || ""} disabled />
            </Field>
          ) : (
            <Field
              label="Account reference"
              hint={
                refMode === "required" ? "Required: this PayBill has no default account. Up to 12 characters"
                : channel?.type === "paybill" ? `Optional, defaults to ${channel.account_number}`
                : "Optional, up to 12 characters"
              }
            >
              <input
                className="input"
                maxLength={12}
                placeholder={channel?.type === "paybill" && channel.account_number ? channel.account_number : "ORDER123"}
                value={form.account_reference}
                onChange={set("account_reference")}
                required={refMode === "required"}
              />
            </Field>
          )}
          <Field label="Description" hint="Optional, up to 13 characters">
            <input className="input" maxLength={13} placeholder="Payment" value={form.description} onChange={set("description")} />
          </Field>
        </div>
        <ErrorText error={error} />
        <div className={styles.modalFooter}>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={submitting}>{submitting ? <Busy>Sending…</Busy> : "Send prompt"}</button>
        </div>
      </form>
    </Modal>
  );
}
