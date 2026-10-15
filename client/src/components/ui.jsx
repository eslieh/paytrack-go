"use client";

import { useState } from "react";
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
      {copied ? <Check size={16} /> : <Copy size={16} />}
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
