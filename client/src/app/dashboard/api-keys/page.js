"use client";

import { useEffect, useState } from "react";
import Tabs from "@/components/Tabs.jsx";
import styles from "@/app/dashboard/styles.module.css";
import { apiFetch } from "@/lib/api";

export default function ApiKeysPage() {
  const [keyData, setKeyData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  async function fetchKey() {
    setError("");
    setLoading(true);
    try {
      const res = await apiFetch(`/tenants/dashboard/key`, { method: "GET" });
      setKeyData(res);
    } catch (e) {
      setKeyData(null);
      setError(e.message || "Failed to fetch key");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { fetchKey(); }, []);

  async function generate() {
    setSaving(true); setError("");
    try {
      const res = await apiFetch(`/tenants/dashboard/key`, { method: "POST", body: JSON.stringify({ action: "generate" }) });
      setKeyData(res);
    } catch (e) { setError(e.message || "Failed to generate key"); }
    finally { setSaving(false); }
  }

  async function regenerate() {
    setSaving(true); setError("");
    try {
      const res = await apiFetch(`/tenants/dashboard/key`, { method: "PATCH", body: JSON.stringify({ action: "regenerate" }) });
      setKeyData(res);
    } catch (e) { setError(e.message || "Failed to regenerate key"); }
    finally { setSaving(false); }
  }

  async function revoke() {
    setSaving(true); setError("");
    try {
      await apiFetch(`/tenants/dashboard/key`, { method: "DELETE" });
      setKeyData(null);
    } catch (e) { setError(e.message || "Failed to delete key"); }
    finally { setSaving(false); }
  }

  function maskKey(value) {
    if (!value) return "";
    if (value.length <= 8) return "*".repeat(value.length);
    return `${value.slice(0,4)}${"*".repeat(value.length - 8)}${value.slice(-4)}`;
  }

  async function copyKey() {
    if (!keyData?.key) return;
    try {
      await navigator.clipboard.writeText(keyData.key);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {}
  }

  return (
    <div className={styles.shell}>
      <Tabs
        activeHref="/dashboard/api-keys"
        items={[
          { label: "Home", href: "/dashboard" },
          { label: "Wallet", href: "/dashboard/wallet" },
          { label: "API Keys", href: "/dashboard/api-keys" },
        ]}
        brand="Paytrack"
      />
      <main className={styles.main}>
        <section className={styles.card}>
          <div className={styles.rowBetween}>
            <div className={styles.h2}>API Keys</div>
            <div style={{display: "flex", gap: 8}}>
              <button className="btn btn-ghost" onClick={fetchKey} disabled={loading || saving}>Refresh</button>
              {keyData ? (
                <>
                  <button className="btn btn-ghost" onClick={regenerate} disabled={saving}>{saving ? "Working..." : "Regenerate"}</button>
                  <button className="btn btn-ghost" onClick={revoke} disabled={saving}>Delete</button>
                </>
              ) : (
                <button className="btn btn-primary" onClick={generate} disabled={saving}>{saving ? "Working..." : "Generate key"}</button>
              )}
            </div>
          </div>

          {loading && (<>
            <div className={styles.skeletonLine} />
            <div className={styles.skeletonLine} />
          </>)}

          {!loading && keyData && (
            <div style={{display: "grid", gap: 12, marginTop: 8}}>
              <div>
                <div className={styles.muted} style={{fontSize: 12}}>Key</div>
                <div className={styles.mono}>{maskKey(keyData.key)}</div>
                <div style={{marginTop: 6}}>
                  <button className="btn btn-primary" onClick={copyKey} disabled={!keyData?.key}>
                    {copied ? "Copied" : "Copy key"}
                  </button>
                </div>
              </div>
              <div style={{display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8}}>
                <div>
                  <div className={styles.muted} style={{fontSize: 12}}>Created</div>
                  <div>{new Date(keyData.created_at).toLocaleString()}</div>
                </div>
                <div>
                  <div className={styles.muted} style={{fontSize: 12}}>Revoked</div>
                  <div>{keyData.revoked_at ? new Date(keyData.revoked_at).toLocaleString() : "—"}</div>
                </div>
                <div>
                  <div className={styles.muted} style={{fontSize: 12}}>Status</div>
                  <div><span className={styles.badgeSuccess}>{keyData.revoked_at ? "revoked" : "active"}</span></div>
                </div>
              </div>
            </div>
          )}

          {!loading && !keyData && (
            <div style={{marginTop: 8}}>
              <div className={styles.muted}>No key found.</div>
            </div>
          )}

          {error && (<div style={{color: "#b42318", fontSize: 13, marginTop: 8}}>{error}</div>)}
        </section>
      </main>
    </div>
  );
}


