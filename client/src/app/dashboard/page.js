"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { CheckCircle2, Circle } from "lucide-react";
import styles from "./styles.module.css";
import { api } from "@/lib/api";
import { destinationLabel, fmtDate, fmtKES, maskPhone } from "@/lib/format";
import dynamic from "next/dynamic";
import { ErrorText, Skeleton, SkeletonRows, StatusBadge } from "@/components/ui.jsx";

// The card (3D tilt, SVG chip) is the heaviest thing on the page: load it lazily,
// with a card-shaped skeleton in its place.
const PaymentCard = dynamic(() => import("@/components/PaymentCard.jsx"), {
  ssr: false,
  loading: () => <Skeleton w="100%" h="auto" r={20} style={{ aspectRatio: "1.586", maxWidth: 400, justifySelf: "center" }} />,
});

const SAMPLE = 100; // payments used for the summary tiles

export default function OverviewPage() {
  const [me, setMe] = useState(null);
  const [channels, setChannels] = useState(null);
  const [payments, setPayments] = useState(null);
  const [onboarding, setOnboarding] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let mounted = true;
    const settle = (p, set) => p.then((v) => mounted && set(v)).catch((e) => mounted && setError(e));
    settle(api.auth.me(), setMe);
    settle(api.channels.list().then((r) => r.data), setChannels);
    settle(api.payments.list({ limit: SAMPLE }).then((r) => r.data), setPayments);
    settle(api.onboarding(), setOnboarding);
    return () => { mounted = false; };
  }, []);

  const succeeded = payments?.filter((p) => p.status === "succeeded") || [];
  const volume = succeeded.reduce((sum, p) => sum + (p.amount || 0), 0);
  const settledCount = payments?.filter((p) => !["created", "pending"].includes(p.status)).length || 0;
  const successRate = settledCount ? Math.round((succeeded.length / settledCount) * 100) : null;

  const steps = onboarding?.steps || [];
  const doneCount = steps.filter((s) => s.done).length;

  return (
    <>
      <section className={styles.card}>
        <div className={styles.kicker}>Business</div>
        <h2 className={styles.h2}>{me ? me.tenant.name : <Skeleton w={220} h={22} style={{ marginTop: 4 }} />}</h2>
        {me && (
          <div className={styles.muted} style={{ fontSize: 13, marginTop: 4 }}>
            <span className={styles.mono} style={{ letterSpacing: 0 }}>{me.tenant.id}</span>
            {me.user && <> · {me.user.email} ({me.user.role})</>}
          </div>
        )}
        <ErrorText error={error} />
      </section>

      <section className={styles.hero}>
        <PaymentCard channels={channels} holder={me?.tenant?.name} payments={payments} />
        <div className={styles.heroStats}>
        <div className={styles.summaryCard}>
          <div className={styles.summaryLabel}>Collected (last {SAMPLE} payments)</div>
          <div className={styles.summaryValue}>{payments ? fmtKES(volume) : <Skeleton w="55%" h={22} />}</div>
        </div>
        <div className={styles.summaryCard}>
          <div className={styles.summaryLabel}>Successful payments</div>
          <div className={styles.summaryValue}>{payments ? succeeded.length : <Skeleton w="25%" h={22} />}</div>
        </div>
        <div className={styles.summaryCard}>
          <div className={styles.summaryLabel}>Success rate</div>
          <div className={styles.summaryValue}>{!payments ? <Skeleton w="30%" h={22} /> : successRate == null ? "—" : `${successRate}%`}</div>
        </div>
        </div>
      </section>

      {onboarding && !onboarding.complete && (
        <section className={styles.card}>
          <div className={styles.rowBetween} style={{ marginBottom: 8 }}>
            <div style={{ fontWeight: 700 }}>Get set up</div>
            <span className={styles.muted} style={{ fontSize: 13 }}>{doneCount} of {steps.length} done</span>
          </div>
          <div style={{ height: 6, borderRadius: 6, background: "#eef0fb", marginBottom: 8, overflow: "hidden" }}>
            <div style={{ height: "100%", width: `${(doneCount / steps.length) * 100}%`, background: "#6472f3", borderRadius: 6, transition: "width .3s" }} />
          </div>
          {steps.map((s) => (
            <Link key={s.href} href={s.href} className={styles.kv} style={{ justifyContent: "flex-start", alignItems: "center" }}>
              {s.done ? <CheckCircle2 size={18} color="#127a3a" /> : <Circle size={18} color="#8a91ad" />}
              <span style={{ textDecoration: s.done ? "line-through" : "none", color: s.done ? "#657090" : undefined }}>{s.label}</span>
            </Link>
          ))}
        </section>
      )}

      <section className={styles.card}>
        <div className={styles.rowBetween}>
          <div style={{ fontWeight: 700 }}>Recent payments</div>
          <Link href="/dashboard/payments" className="btn btn-ghost" style={{ padding: "6px 10px" }}>View all</Link>
        </div>
        <div className={styles.table} style={{ "--cols": "1.6fr 1fr 1fr 1.2fr" }}>
          <div className={styles.thead}>
            <div>Reference</div><div>Amount</div><div>Status</div><div>Date</div>
          </div>
          <div className={styles.tbody}>
            {!payments && <SkeletonRows rows={5} cols={4} />}
            {payments?.length === 0 && <div className={styles.empty}>No payments yet.</div>}
            {payments?.slice(0, 5).map((p) => (
              <Link key={p.id} href={`/dashboard/payments?id=${encodeURIComponent(p.id)}`} className={`${styles.trow} ${styles.clickRow}`}>
                <div>
                  <div className={styles.cellTitle}>{p.account_reference}</div>
                  <div className={styles.cellSub}>{maskPhone(p.phone)}</div>
                </div>
                <div>{fmtKES(p.amount)}</div>
                <div><StatusBadge status={p.status} /></div>
                <div className={styles.cellSub}>{fmtDate(p.created_at)}</div>
              </Link>
            ))}
          </div>
        </div>
      </section>

      {channels && channels.length > 0 && (
        <section className={styles.card}>
          <div className={styles.rowBetween}>
            <div style={{ fontWeight: 700 }}>Channels</div>
            <Link href="/dashboard/channels" className="btn btn-ghost" style={{ padding: "6px 10px" }}>Manage</Link>
          </div>
          {channels.map((c) => (
            <div key={c.id} className={styles.kv}>
              <span>{c.name} <span className={styles.cellSub}>· {destinationLabel(c)}</span></span>
              <span><StatusBadge status={c.active ? "active" : "inactive"} tone={c.active ? "success" : "neutral"} /></span>
            </div>
          ))}
        </section>
      )}
    </>
  );
}
