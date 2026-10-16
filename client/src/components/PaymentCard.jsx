"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Plus, RotateCw } from "lucide-react";
import styles from "./paymentCard.module.css";
import { CopyButton } from "./ui.jsx";
import { fmtKES } from "@/lib/format";

const TYPE_LABEL = { bank: "BANK", paybill: "PAYBILL", till: "TILL" };

// The "card number": the number money is routed to, in groups of four.
function primaryNumber(c) {
  return c.type === "bank" ? c.account_number || "" : c.business_number || "";
}

// Group in fours from the right, so the last four always sit together like on a real card.
function groupFromEnd(raw) {
  const out = [];
  for (let end = raw.length; end > 0; end -= 4) out.unshift(raw.slice(Math.max(0, end - 4), end));
  return out.join(" ");
}

// Bank account numbers are private: mask all but the last four.
// PayBill and Till numbers are public business numbers: show them in full, 3-3 / 3-4.
function displayNumber(c) {
  const raw = primaryNumber(c).replace(/\s+/g, "");
  if (!raw) return "•••• ••••";
  if (c.type !== "bank") return raw.replace(/^(\d{3})(\d+)$/, "$1 $2");
  if (raw.length <= 4) return raw;
  return groupFromEnd("•".repeat(raw.length - 4) + raw.slice(-4));
}

function since(iso) {
  if (!iso) return "--/--";
  const d = new Date(iso);
  return `${String(d.getMonth() + 1).padStart(2, "0")}/${String(d.getFullYear()).slice(-2)}`;
}

function subtitle(c) {
  if (c.type === "bank") return `${c.bank_name || "Bank"} · PayBill ${c.business_number}`;
  if (c.type === "paybill") return c.account_number ? `Default account ${c.account_number}` : "Account set per payment";
  return "Buy Goods";
}

function Chip() {
  return (
    <svg className={styles.chip} viewBox="0 0 50 38" aria-hidden="true">
      <defs>
        <linearGradient id="chipGold" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#f6e27a" />
          <stop offset="0.5" stopColor="#d4a93c" />
          <stop offset="1" stopColor="#f3d26b" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="48" height="36" rx="7" fill="url(#chipGold)" stroke="#b8902f" strokeWidth="1" />
      <path d="M1 13h14M1 25h14M35 13h14M35 25h14M15 1v36M35 1v36M15 19h20" stroke="#a57f26" strokeWidth="1.2" fill="none" opacity="0.75" />
    </svg>
  );
}

function Contactless() {
  return (
    <svg className={styles.contactless} viewBox="0 0 24 24" aria-hidden="true">
      {[5, 9, 13].map((r, i) => (
        <path key={r} d={`M${6 + i * 3} ${12 - r / 2} a ${r} ${r} 0 0 1 0 ${r}`} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      ))}
    </svg>
  );
}

export default function PaymentCard({ channels, holder, payments }) {
  const active = (channels || []).filter((c) => c.active);
  const [i, setI] = useState(0);
  const [flipped, setFlipped] = useState(false);
  const [tilt, setTilt] = useState({ x: 0, y: 0, mx: 50, my: 50 });
  const ref = useRef(null);

  if (!channels) return <div className={`${styles.card} ${styles.skeleton}`} />;

  if (active.length === 0) {
    return (
      <Link href="/dashboard/channels" className={`${styles.card} ${styles.empty}`}>
        <Plus size={28} />
        <strong>Add your first channel</strong>
        <span>Bank account, PayBill or Till. It shows up here as a card.</span>
      </Link>
    );
  }

  const c = active[i % active.length];
  const mine = (payments || []).filter((p) => p.channel_id === c.id);
  const collected = mine.filter((p) => p.status === "succeeded").reduce((s, p) => s + (p.amount || 0), 0);

  function onMove(e) {
    const r = ref.current?.getBoundingClientRect();
    if (!r) return;
    const px = (e.clientX - r.left) / r.width;
    const py = (e.clientY - r.top) / r.height;
    setTilt({ x: (0.5 - py) * 12, y: (px - 0.5) * 16, mx: px * 100, my: py * 100 });
  }

  const go = (d) => { setFlipped(false); setI((v) => (v + d + active.length) % active.length); };

  return (
    <div className={styles.wrap}>
      <div
        key={c.id /* remount on channel switch so the swap animation replays */}
        ref={ref}
        className={styles.scene}
        onMouseMove={onMove}
        onMouseLeave={() => setTilt({ x: 0, y: 0, mx: 50, my: 50 })}
        onClick={() => setFlipped((v) => !v)}
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setFlipped((v) => !v); } }}
        role="button"
        tabIndex={0}
        aria-label={`${c.name} card. Press to ${flipped ? "show the front" : "show details"}.`}
        style={{ "--rx": `${tilt.x}deg`, "--ry": `${tilt.y + (flipped ? 180 : 0)}deg`, "--mx": `${tilt.mx}%`, "--my": `${tilt.my}%` }}
      >
        <div className={styles.inner}>
          {/* front */}
          <div className={`${styles.card} ${styles.face} ${styles[c.type]}`}>
            <div className={styles.pattern} />
            <div className={styles.shine} />
            <div className={styles.row}>
              <div>
                <div className={styles.channelName}>{c.name}</div>
                <div className={styles.sub}>{subtitle(c)}</div>
              </div>
              <span className={styles.typeTag}>{TYPE_LABEL[c.type]}</span>
            </div>
            <div className={styles.chipRow}>
              <Chip />
              <Contactless />
            </div>
            <div className={styles.number}>{displayNumber(c)}</div>
            <div className={styles.row} style={{ alignItems: "flex-end" }}>
              <div className={styles.holderBlock}>
                <span className={styles.tiny}>Account holder</span>
                <span className={styles.holder}>{(holder || "Your business").toUpperCase()}</span>
              </div>
              <div className={styles.holderBlock}>
                <span className={styles.tiny}>Since</span>
                <span className={styles.holder}>{since(c.created_at)}</span>
              </div>
              <span className={styles.network}>paytrack</span>
            </div>
          </div>

          {/* back */}
          <div className={`${styles.card} ${styles.face} ${styles.back} ${styles[c.type]}`}>
            <div className={styles.stripe} />
            <div className={styles.sigRow} onClick={(e) => e.stopPropagation()}>
              <code className={styles.sig}>{c.id}</code>
              <CopyButton text={c.id} label="ID" />
            </div>
            <div className={styles.backGrid}>
              <div><span className={styles.tiny}>Routes to</span><span className={styles.backVal}>{c.type === "bank" ? c.bank_name : `${TYPE_LABEL[c.type]} ${c.business_number}`}</span></div>
              <div><span className={styles.tiny}>{c.type === "till" ? "Till" : "Account"}</span><span className={styles.backVal}>{c.type === "till" ? c.business_number : c.account_number || "per payment"}</span></div>
              <div><span className={styles.tiny}>Collected*</span><span className={styles.backVal}>{fmtKES(collected)}</span></div>
              <div><span className={styles.tiny}>Payments*</span><span className={styles.backVal}>{mine.length}</span></div>
            </div>
            <div className={styles.fine}>* from your latest payments. Money settles straight to this destination; Paytrack never holds it.</div>
          </div>
        </div>
      </div>

      <div className={styles.controls}>
        {active.length > 1 && (
          <>
            <button onClick={() => go(-1)} aria-label="Previous channel"><ChevronLeft size={16} /></button>
            <div className={styles.dots}>
              {active.map((x, j) => (
                <button key={x.id} className={j === i % active.length ? styles.dotOn : ""} onClick={() => { setFlipped(false); setI(j); }} aria-label={`Show ${x.name}`} />
              ))}
            </div>
            <button onClick={() => go(1)} aria-label="Next channel"><ChevronRight size={16} /></button>
          </>
        )}
        <button className={styles.flipBtn} onClick={() => setFlipped((v) => !v)}>
          <RotateCw size={14} /> {flipped ? "Front" : "Details"}
        </button>
      </div>
    </div>
  );
}
