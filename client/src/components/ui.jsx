"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";
import styles from "../app/dashboard/styles.module.css";
import { copyText, statusTone } from "../lib/format";

const toneClass = {
  success: styles.badgeSuccess,
  danger: styles.badgeDanger,
  warning: styles.badgeWarning,
  neutral: styles.pill,
};

export function StatusBadge({ status, tone }) {
  return <span className={toneClass[tone || statusTone(status)]}>{status}</span>;
}

export function ErrorText({ error }) {
  if (!error) return null;
  const msg = typeof error === "string" ? error : error.message;
  return (
    <div className={styles.error}>
      {msg}
      {error.requestId && <span className={styles.muted}> (request {error.requestId})</span>}
    </div>
  );
}

export function CopyButton({ text, label = "Copy" }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      className="btn btn-ghost"
      style={{ padding: "6px 10px" }}
      disabled={!text}
      onClick={async () => {
        if (await copyText(text)) {
          setCopied(true);
          setTimeout(() => setCopied(false), 1200);
        }
      }}
    >
      <span key={copied ? "y" : "n"} className="icon-pop">{copied ? <Check size={16} /> : <Copy size={16} />}</span>
      {copied ? "Copied" : label}
    </button>
  );
}

export function Modal({ title, onClose, children, footer }) {
  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <div className={styles.modalTitle}>{title}</div>
        <div>{children}</div>
        {footer && <div className={styles.modalFooter}>{footer}</div>}
      </div>
    </div>
  );
}

export function Field({ label, hint, children }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      {children}
      {hint && <span className={styles.fieldHint}>{hint}</span>}
    </label>
  );
}

export function KV({ label, children }) {
  return (
    <div className={styles.kv}>
      <span className={styles.muted}>{label}</span>
      <span>{children ?? "—"}</span>
    </div>
  );
}

// ---------------------------------------------------------------- loading states

// Skeleton is a shimmering placeholder block. Size it like the content it replaces.
export function Skeleton({ w = "100%", h = 14, r = 6, style }) {
  return <span className={styles.skeleton} style={{ width: w, height: h, borderRadius: r, ...style }} aria-hidden="true" />;
}

// SkeletonRows mimics table rows: one bar per column, the first column two lines
// (title + subtitle) like the real rows. Widths vary so it doesn't look like a grid of bricks.
const WIDTHS = ["72%", "54%", "64%", "46%", "58%"];
export function SkeletonRows({ rows = 4, cols = 4 }) {
  return Array.from({ length: rows }, (_, r) => (
    <div key={r} className={styles.trow} aria-hidden="true" style={{ animationDelay: `${r * 60}ms` }}>
      {Array.from({ length: cols }, (_, c) => (
        <div key={c} style={{ display: "grid", gap: 6 }}>
          <Skeleton w={WIDTHS[(r + c) % WIDTHS.length]} h={c === 0 ? 13 : 12} />
          {c === 0 && <Skeleton w="40%" h={10} />}
        </div>
      ))}
    </div>
  ));
}

// SkeletonKV mimics a list of label/value rows (detail views, settings).
export function SkeletonKV({ rows = 4 }) {
  return Array.from({ length: rows }, (_, i) => (
    <div key={i} className={styles.kv} aria-hidden="true">
      <Skeleton w={`${24 + ((i * 13) % 20)}%`} h={12} />
      <Skeleton w={`${30 + ((i * 17) % 25)}%`} h={12} />
    </div>
  ));
}

// PageSkeleton stands in for a whole dashboard page before the session is ready.
export function PageSkeleton() {
  return (
    <div className="page-enter" role="status" aria-label="Loading">
      <section className={styles.card} style={{ display: "grid", gap: 10 }}>
        <Skeleton w={90} h={10} />
        <Skeleton w={220} h={22} />
      </section>
      <section className={styles.summaryGrid} style={{ marginTop: 0 }}>
        {[0, 1, 2].map((i) => (
          <div key={i} className={styles.summaryCard} style={{ display: "grid", gap: 8 }}>
            <Skeleton w="60%" h={10} />
            <Skeleton w="45%" h={20} />
          </div>
        ))}
      </section>
      <section className={styles.card}>
        <Skeleton w={140} h={16} />
        <div className={styles.table} style={{ "--cols": "1.6fr 1fr 1fr 1.2fr" }}>
          <div className={styles.tbody}><SkeletonRows rows={5} cols={4} /></div>
        </div>
      </section>
    </div>
  );
}

// useInView calls onVisible whenever `ref` scrolls within `margin` of the viewport.
// Used to lazy-load the next page of a list before the user reaches the end.
export function useInView(ref, onVisible, { enabled = true, margin = "400px" } = {}) {
  const cb = useRef(onVisible);
  cb.current = onVisible;
  useEffect(() => {
    const el = ref.current;
    if (!el || !enabled || typeof IntersectionObserver === "undefined") return;
    const io = new IntersectionObserver((entries) => entries[0]?.isIntersecting && cb.current(), { rootMargin: margin });
    io.observe(el);
    return () => io.disconnect();
  }, [ref, enabled, margin]);
}
