"use client";

// Shared building blocks for the auth screens, in the same style as the
// existing sign-in card (globals.css .card/.input/.btn).
import { useEffect, useRef, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { api, googleStartUrl } from "@/lib/api";

export const labelStyle = { fontSize: 13, color: "#555b73", fontWeight: 600 };
const iconStyle = { position: "absolute", left: 12, top: 12 };

export function IconField({ id, label, icon: Icon, children, hint }) {
  return (
    <label className="stack" htmlFor={id} style={{ gap: 6 }}>
      <span style={labelStyle}>{label}</span>
      <div style={{ position: "relative", width: "100%" }}>
        {Icon && <Icon size={18} color="#7b82a1" style={iconStyle} />}
        {children}
      </div>
      {hint && <span style={{ fontSize: 12, color: "#8a91ad" }}>{hint}</span>}
    </label>
  );
}

export function PasswordInput({ id, value, onChange, autoComplete = "current-password", placeholder = "••••••••", minLength }) {
  const [show, setShow] = useState(false);
  return (
    <>
      <input
        id={id}
        type={show ? "text" : "password"}
        autoComplete={autoComplete}
        className="input"
        placeholder={placeholder}
        style={{ paddingLeft: 38, paddingRight: 38 }}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        minLength={minLength}
        maxLength={72}
        required
      />
      <button
        type="button"
        onClick={() => setShow((v) => !v)}
        aria-label={show ? "Hide password" : "Show password"}
        style={{ position: "absolute", right: 12, top: 12, background: "none", border: 0, padding: 0, cursor: "pointer" }}
      >
        {show ? <EyeOff size={18} color="#7b82a1" /> : <Eye size={18} color="#7b82a1" />}
      </button>
    </>
  );
}

// passwordStrength scores 0-4 for the meter on sign-up/reset.
export function passwordStrength(pw) {
  let score = 0;
  if (pw.length >= 10) score++;
  if (pw.length >= 14) score++;
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) score++;
  if (/\d/.test(pw) && /[^A-Za-z0-9]/.test(pw)) score++;
  return Math.min(score, 4);
}

export function StrengthMeter({ password }) {
  if (!password) return null;
  const s = passwordStrength(password);
  const colors = ["#ef4444", "#ef4444", "#f59e0b", "#22c55e", "#16a34a"];
  const labels = ["Too short", "Weak", "Okay", "Strong", "Very strong"];
  return (
    <div style={{ display: "grid", gap: 4 }} aria-live="polite">
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 4 }}>
        {[0, 1, 2, 3].map((i) => (
          <div key={i} style={{ height: 4, borderRadius: 4, background: i < s ? colors[s] : "#e9ebf4" }} />
        ))}
      </div>
      <span style={{ fontSize: 12, color: "#657090" }}>{password.length < 10 ? "At least 10 characters" : labels[s]}</span>
    </div>
  );
}

export function ErrorLine({ error }) {
  if (!error) return null;
  return <div role="alert" style={{ color: "#b42318", fontSize: 13 }}>{typeof error === "string" ? error : error.message}</div>;
}

export function SuccessLine({ children }) {
  return (
    <div style={{ background: "#ecfdf3", border: "1px solid #abefc6", color: "#067647", borderRadius: 10, padding: "10px 12px", fontSize: 13 }}>
      {children}
    </div>
  );
}

export function Divider({ label = "or" }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, color: "#8a91ad", fontSize: 12, margin: "4px 0" }}>
      <span style={{ flex: 1, height: 1, background: "#e9ebf4" }} />
      {label}
      <span style={{ flex: 1, height: 1, background: "#e9ebf4" }} />
    </div>
  );
}

function GoogleLogo() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3C33.7 32.7 29.2 36 24 36c-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.4-.4-3.5z" />
      <path fill="#FF3D00" d="M6.3 14.7l6.6 4.8C14.7 15.1 19 12 24 12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z" />
      <path fill="#4CAF50" d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2C29.2 35.1 26.7 36 24 36c-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z" />
      <path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.2-4.1 5.6l6.2 5.2C37 39.2 44 34 44 24c0-1.3-.1-2.4-.4-3.5z" />
    </svg>
  );
}

// GoogleButton renders only when the API has Google sign-in configured.
export function GoogleButton({ next = "/dashboard", label = "Continue with Google" }) {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    api.auth.providers().then((p) => setEnabled(Boolean(p.google))).catch(() => setEnabled(false));
  }, []);
  if (!enabled) return null;
  return (
    <>
      <a
        href={googleStartUrl(next)}
        className="btn"
        style={{ border: "1px solid #e5e7ef", background: "#fff", color: "#0f1222", width: "100%" }}
      >
        <GoogleLogo />
        {label}
      </a>
      <Divider />
    </>
  );
}

// OtpInput is a 6-box code input with paste and auto-advance.
export function OtpInput({ value, onChange, onComplete, disabled }) {
  const refs = useRef([]);
  const digits = value.padEnd(6, " ").slice(0, 6).split("");

  function set(i, d) {
    const next = (value.slice(0, i) + d + value.slice(i + 1)).replace(/\s/g, "").slice(0, 6);
    onChange(next);
    if (next.length === 6) onComplete?.(next);
  }

  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(6, 1fr)", gap: 8 }}>
      {digits.map((d, i) => (
        <input
          key={i}
          ref={(el) => (refs.current[i] = el)}
          inputMode="numeric"
          autoComplete={i === 0 ? "one-time-code" : "off"}
          aria-label={`Digit ${i + 1}`}
          maxLength={6}
          disabled={disabled}
          value={d.trim()}
          className="input"
          style={{ textAlign: "center", fontSize: 22, fontWeight: 700, padding: "12px 0", fontFamily: "ui-monospace, Menlo, monospace" }}
          onChange={(e) => {
            const v = e.target.value.replace(/\D/g, "");
            if (!v) return;
            if (v.length > 1) { // paste or autofill
              const all = v.slice(0, 6);
              onChange(all);
              refs.current[Math.min(all.length, 5)]?.focus();
              if (all.length === 6) onComplete?.(all);
              return;
            }
            set(i, v);
            refs.current[i + 1]?.focus();
          }}
          onKeyDown={(e) => {
            if (e.key === "Backspace" && !d.trim() && i > 0) {
              onChange(value.slice(0, i - 1));
              refs.current[i - 1]?.focus();
            }
          }}
          autoFocus={i === 0}
        />
      ))}
    </div>
  );
}

// useCountdown drives resend buttons.
export function useCountdown(seconds) {
  const [left, setLeft] = useState(seconds);
  useEffect(() => {
    if (left <= 0) return;
    const t = setTimeout(() => setLeft((v) => v - 1), 1000);
    return () => clearTimeout(t);
  }, [left]);
  return [left, () => setLeft(seconds)];
}

// Messages for ?error= codes the API redirects back with.
export const AUTH_ERRORS = {
  google_cancelled: "Google sign-in was cancelled.",
  google_state_invalid: "Google sign-in expired or was started in another browser. Please try again.",
  google_failed: "Google sign-in failed. Please try again.",
  google_email_unverified: "Your Google account's email isn't verified.",
  google_disabled: "Google sign-in isn't available right now.",
  google_in_use: "That Google account is linked to another Paytrack account.",
};

// safeNext only follows same-site relative redirects.
export function safeNext(next) {
  return next && next.startsWith("/") && !next.startsWith("//") ? next : "/dashboard";
}
