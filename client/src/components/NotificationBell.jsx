"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell } from "lucide-react";
import { api } from "../lib/api";
import { Busy } from "./Busy.jsx";

const POLL_MS = 60_000;

// useUnread polls the unread count every minute and when the tab regains
// focus. refresh() is for after the user reads something.
export function useUnread(enabled) {
  const [unread, setUnread] = useState(0);
  const refresh = useCallback(() => {
    api.inbox.unreadCount().then((r) => setUnread(r.unread)).catch(() => {});
  }, []);
  useEffect(() => {
    if (!enabled) return;
    refresh();
    const t = setInterval(refresh, POLL_MS);
    const onFocus = () => document.visibilityState === "visible" && refresh();
    document.addEventListener("visibilitychange", onFocus);
    return () => { clearInterval(t); document.removeEventListener("visibilitychange", onFocus); };
  }, [enabled, refresh]);
  return [unread, refresh];
}

function ago(s) {
  const sec = Math.max(0, (Date.now() - new Date(s).getTime()) / 1000);
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

// NotificationBell shows the unread badge and, on click, the latest
// notifications. The panel is fixed-positioned so the scrolling sidebar
// can't clip it.
export default function NotificationBell({ unread, onChanged }) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState(null);
  const [pos, setPos] = useState(null);
  const btn = useRef(null);
  const panel = useRef(null);
  const router = useRouter();

  async function toggle() {
    if (open) { setOpen(false); return; }
    const r = btn.current.getBoundingClientRect();
    const width = Math.min(360, window.innerWidth - 20);
    setPos({ top: r.bottom + 8, left: Math.max(10, Math.min(r.left, window.innerWidth - width - 10)), width });
    setOpen(true);
    setItems(null);
    try { setItems((await api.inbox.list({ limit: 20 })).data); } catch { setItems([]); }
  }

  useEffect(() => {
    if (!open) return;
    const close = (e) => {
      if (!panel.current?.contains(e.target) && !btn.current?.contains(e.target)) setOpen(false);
    };
    const esc = (e) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", esc);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", esc); };
  }, [open]);

  async function openItem(n) {
    if (!n.read_at) {
      setItems((cur) => cur.map((x) => (x.id === n.id ? { ...x, read_at: new Date().toISOString() } : x)));
      try { await api.inbox.markRead(n.id); } catch {}
      onChanged();
    }
    if (n.link) { setOpen(false); router.push(n.link); }
  }

  async function readAll() {
    try { await api.inbox.markAllRead(); } catch {}
    setItems((cur) => cur?.map((x) => ({ ...x, read_at: x.read_at || new Date().toISOString() })));
    onChanged();
  }

  return (
    <>
      <button ref={btn} onClick={toggle} aria-label={unread ? `Notifications, ${unread} unread` : "Notifications"} aria-expanded={open}
        style={{ position: "relative", display: "inline-flex", padding: 6, borderRadius: 8, border: "1px solid #2a2f50", background: "#171a2b", color: "#eef0f6", cursor: "pointer" }}>
        <Bell size={16} />
        {unread > 0 && (
          <span style={{ position: "absolute", top: -6, right: -6, minWidth: 18, height: 18, padding: "0 5px", borderRadius: 9, background: "#f04438", color: "#fff", fontSize: 11, fontWeight: 700, display: "grid", placeItems: "center", lineHeight: 1 }}>
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </button>

      {open && pos && (
        <div ref={panel} role="dialog" aria-label="Notifications"
          style={{ position: "fixed", top: pos.top, left: pos.left, width: pos.width, maxHeight: "min(480px, 70vh)", overflowY: "auto", zIndex: 60, background: "#fff", color: "#0f1222", border: "1px solid #e9ebf4", borderRadius: 14, boxShadow: "0 16px 40px rgba(12, 18, 37, 0.18)" }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 14px", borderBottom: "1px solid #f1f3fb", position: "sticky", top: 0, background: "#fff" }}>
            <strong style={{ fontSize: 14 }}>Notifications</strong>
            {items?.some((n) => !n.read_at) && (
              <button className="btn btn-ghost" style={{ padding: 0, fontSize: 12 }} onClick={readAll}>Mark all read</button>
            )}
          </div>
          {!items && <div style={{ padding: 16, fontSize: 13 }}><Busy>Loading…</Busy></div>}
          {items?.length === 0 && <div style={{ padding: 24, fontSize: 13, color: "#657090", textAlign: "center" }}>You&apos;re all caught up.</div>}
          {items?.map((n) => (
            <button key={n.id} onClick={() => openItem(n)}
              style={{ display: "grid", gap: 2, width: "100%", textAlign: "left", padding: "12px 14px", border: 0, borderBottom: "1px solid #f1f3fb", background: n.read_at ? "#fff" : "#f5f6ff", cursor: n.link || !n.read_at ? "pointer" : "default", font: "inherit", color: "inherit" }}>
              <span style={{ display: "flex", gap: 8, alignItems: "baseline", justifyContent: "space-between" }}>
                <span style={{ fontSize: 13, fontWeight: n.read_at ? 500 : 700 }}>{n.title}</span>
                <span style={{ fontSize: 11, color: "#8a91ad", whiteSpace: "nowrap" }}>{ago(n.created_at)}</span>
              </span>
              {n.body && <span style={{ fontSize: 12, color: "#657090", overflowWrap: "anywhere" }}>{n.body}</span>}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
