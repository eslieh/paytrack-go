"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtKES } from "@/lib/format";
import { ErrorText, Skeleton } from "@/components/ui.jsx";
import { BarList, ColumnChart, compact, eachDay } from "@/components/charts.jsx";

const RANGES = [[7, "7 days"], [30, "30 days"], [90, "90 days"]];
const KINDS = ["payments", "webhooks", "tenants", "users", "emails", "support"];
const DEST_LABEL = { bank: "Bank accounts", paybill: "PayBills", till: "Tills" };

// nairobiDate turns the API's window start (midnight EAT) into YYYY-MM-DD.
const nairobiDate = (iso) => new Date(new Date(iso).getTime() + 3 * 3600_000).toISOString().slice(0, 10);
const pct = (num, den) => (den ? `${Math.round((num / den) * 1000) / 10}%` : "—");
const kes = (n) => `KES ${compact(n)}`;

function duration(sec) {
  if (sec == null) return "—";
  if (sec < 3600) return `${Math.max(1, Math.round(sec / 60))}m`;
  if (sec < 86400) return `${Math.round((sec / 3600) * 10) / 10}h`;
  return `${Math.round((sec / 86400) * 10) / 10}d`;
}

function Tile({ label, value, sub, href }) {
  const body = (
    <>
      <div style={{ fontSize: 13, color: "#657090" }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 700, marginTop: 4, color: "#0f1222" }}>{value}</div>
      {sub && <div style={{ fontSize: 12, color: "#8a91ad", marginTop: 2 }}>{sub}</div>}
    </>
  );
  const style = { border: "1px solid #e9ebf4", borderRadius: 14, padding: 14, background: "#fff", color: "inherit", textDecoration: "none", display: "block" };
  return href ? <Link href={href} style={style}>{body}</Link> : <div style={style}>{body}</div>;
}

export default function OverviewPage() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    Promise.all(KINDS.map((k) => api.admin.stats(k, days)))
      .then((rs) => setData({ ...Object.fromEntries(KINDS.map((k, i) => [k, rs[i].stats])), since: rs[0].since, days }))
      .catch(setError)
      .finally(() => setLoading(false));
  }, [days]);

  return (
    <>
      {/* Filters: one row above everything they scope. */}
      <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
        {RANGES.map(([d, label]) => (
          <button key={d} className={`btn ${days === d ? "btn-primary" : ""}`} style={{ padding: "6px 12px", fontSize: 13 }} onClick={() => setDays(d)}>
            Last {label}
          </button>
        ))}
        <span className={styles.muted} style={{ fontSize: 12, marginLeft: 6 }}>Days are in Nairobi time.</span>
      </div>
      <ErrorText error={error} />
      {!data ? <OverviewSkeleton /> : (
        // Refetch keeps the frame: previous numbers stay, dimmed.
        <div style={{ display: "grid", gap: 16, opacity: loading ? 0.5 : 1, transition: "opacity 150ms" }}>
          <Body d={data} />
        </div>
      )}
    </>
  );
}

function Body({ d }) {
  const p = d.payments, w = d.webhooks;
  const finished = (p.counts.succeeded || 0) + (p.counts.failed || 0) + (p.counts.cancelled || 0) + (p.counts.expired || 0);
  const delivered = w.counts.delivered || 0, wFailed = w.counts.failed || 0;
  const since = nairobiDate(d.since);
  const volume = eachDay(since, d.days, p.daily, (r) => [r ? r.amount : 0]);
  const hooks = eachDay(since, d.days, w.daily, (r) => [r ? r.delivered : 0, r ? r.failed : 0]);
  const sentMail = d.emails.counts.sent || 0, failedMail = d.emails.counts.failed || 0;

  return (
    <>
      <section className={styles.card}>
        <div style={{ fontSize: 13, color: "#657090" }}>Money routed · last {d.days} days</div>
        <div style={{ fontSize: 48, fontWeight: 700, letterSpacing: "-0.02em", lineHeight: 1.1, color: "#0f1222" }}>{fmtKES(p.routed_amount)}</div>
        <div style={{ fontSize: 13, color: "#657090", marginTop: 4 }}>
          {(p.counts.succeeded || 0).toLocaleString()} successful payments · {fmtKES(p.routed_all_time)} all time
        </div>
        <div style={{ marginTop: 16 }}>
          <ColumnChart title="Money routed per day" data={volume} series={[{ name: "Routed" }]} format={kes} />
        </div>
      </section>

      <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))" }}>
        <Tile label="Payment success rate" value={pct(p.counts.succeeded || 0, finished)} sub={`${finished.toLocaleString()} finished · ${((p.counts.pending || 0) + (p.counts.created || 0)).toLocaleString()} in progress`} />
        <Tile label="Webhook delivery rate" value={pct(delivered, delivered + wFailed)} sub={`${delivered.toLocaleString()} delivered · ${wFailed.toLocaleString()} failed`} />
        <Tile label="Active businesses" value={p.active_tenants.toLocaleString()} sub={`of ${d.tenants.total.toLocaleString()} · ${d.tenants.new} new · ${d.tenants.suspended} suspended`} href="/admin/tenants" />
        <Tile label="Users" value={d.users.total.toLocaleString()} sub={`${d.users.active} signed in · ${d.users.total - d.users.verified} unverified`} />
        <Tile label="Needs reply" value={d.support.awaiting_staff} sub={`${d.support.opened} opened · first reply ${duration(d.support.avg_first_response_seconds)}`} href="/admin/support" />
        <Tile label="Satisfaction" value={d.support.avg_rating ? `${d.support.avg_rating.toFixed(1)} / 5` : "—"} sub={`${d.support.ratings} ratings · ${d.support.feedback} feedback`} href="/admin/feedback" />
      </div>

      <section className={styles.card}>
        <div style={{ fontWeight: 700 }}>Webhook deliveries per day</div>
        <div className={styles.muted} style={{ fontSize: 13, marginBottom: 8 }}>
          {w.failing_tenants ? `${w.failing_tenants} business${w.failing_tenants === 1 ? "" : "es"} had a delivery fail after every retry.` : "No deliveries failed after every retry."}
        </div>
        <ColumnChart title="Webhook deliveries per day" data={hooks} series={[{ name: "Delivered" }, { name: "Failed" }]} format={compact} />
      </section>

      <div style={{ display: "grid", gap: 16, gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))" }}>
        <section className={styles.card}>
          <div style={{ fontWeight: 700, marginBottom: 12 }}>Routed by destination</div>
          {p.by_destination.length === 0 ? <div className={styles.muted} style={{ fontSize: 13 }}>No successful payments yet.</div>
            : <BarList rows={p.by_destination.map((x) => ({ label: `${DEST_LABEL[x.type] || x.type} · ${x.count.toLocaleString()}`, value: x.amount }))} format={kes} />}
        </section>
        <section className={styles.card}>
          <div style={{ fontWeight: 700, marginBottom: 12 }}>Top failure reasons</div>
          {p.top_failures.length === 0 ? <div className={styles.muted} style={{ fontSize: 13 }}>No failed payments.</div>
            : <BarList rows={p.top_failures.map((x) => ({ label: x.reason, value: x.count }))} format={(n) => n.toLocaleString()} />}
        </section>
        <section className={styles.card}>
          <div style={{ fontWeight: 700 }}>Emails</div>
          <div className={styles.muted} style={{ fontSize: 13, marginBottom: 12 }}>
            {sentMail.toLocaleString()} sent · {failedMail.toLocaleString()} failed · {((d.emails.counts.queued || 0) + (d.emails.counts.sending || 0)).toLocaleString()} queued
          </div>
          {d.emails.by_template.length === 0 ? <div className={styles.muted} style={{ fontSize: 13 }}>No emails.</div>
            : <BarList rows={d.emails.by_template.slice(0, 6).map((x) => ({ label: x.template.replaceAll("_", " ") + (x.failed ? ` · ${x.failed} failed` : ""), value: x.sent }))} format={(n) => n.toLocaleString()} />}
        </section>
      </div>
    </>
  );
}

function OverviewSkeleton() {
  return (
    <div style={{ display: "grid", gap: 16 }}>
      <section className={styles.card}><Skeleton w="30%" /><Skeleton w="45%" h={44} style={{ marginTop: 10 }} /><Skeleton h={180} style={{ marginTop: 16 }} /></section>
      <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))" }}>
        {Array.from({ length: 6 }, (_, i) => <section key={i} className={styles.card}><Skeleton w="60%" /><Skeleton w="40%" h={24} style={{ marginTop: 8 }} /></section>)}
      </div>
    </div>
  );
}
