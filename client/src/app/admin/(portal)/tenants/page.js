"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Search } from "lucide-react";
import styles from "@/app/dashboard/styles.module.css";
import { api } from "@/lib/api";
import { fmtDate } from "@/lib/format";
import { ErrorText, SkeletonRows, StatusBadge } from "@/components/ui.jsx";

// Accounts: search businesses by name or ID, or users by email.
export default function AccountsPage() {
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const [tenants, setTenants] = useState(null);
  const [users, setUsers] = useState(null);
  const [cursor, setCursor] = useState(null);
  const [error, setError] = useState(null);
  const byEmail = query.includes("@");

  async function load(after) {
    setError(null);
    try {
      const r = await api.admin.tenants({ q: byEmail ? "" : query, status, cursor: after });
      setTenants((cur) => (after ? [...(cur || []), ...r.data] : r.data));
      setCursor(r.has_more ? r.next_cursor : null);
    } catch (e) {
      setError(e);
    }
  }

  useEffect(() => {
    setTenants(null);
    setUsers(null);
    if (byEmail) {
      api.admin.users({ q: query }).then((r) => setUsers(r.data)).catch(setError);
    } else {
      load();
    }
  }, [query, status]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <section className={styles.card}>
      <div className={styles.h2}>Accounts</div>
      <div className={styles.muted} style={{ fontSize: 13 }}>Search businesses by name or tenant ID, or users by email.</div>
      <form onSubmit={(e) => { e.preventDefault(); setQuery(q.trim()); }} style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
        <div style={{ position: "relative", flex: "1 1 260px" }}>
          <Search size={16} color="#7b82a1" style={{ position: "absolute", left: 12, top: 13 }} />
          <input className="input" style={{ paddingLeft: 36 }} placeholder="Acme, ten_…, or owner@acme.co.ke" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        {!byEmail && (
          <select className={styles.select} style={{ width: "auto" }} value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">All statuses</option>
            <option value="active">Active</option>
            <option value="suspended">Suspended</option>
          </select>
        )}
        <button className="btn btn-primary" type="submit">Search</button>
      </form>

      {byEmail ? (
        <div className={styles.table} style={{ "--cols": "2fr 1.6fr 1fr 1.2fr", "--min-w": "640px" }}>
          <div className={styles.thead}><div>User</div><div>Tenant</div><div>Status</div><div>Last sign-in</div></div>
          <div className={styles.tbody}>
            {!users && !error && <SkeletonRows rows={2} cols={4} />}
            {users?.length === 0 && <div className={styles.empty}>No users match.</div>}
            {users?.map((u) => (
              <Link key={u.id} href={`/admin/users/${u.id}`} className={styles.trow} style={{ color: "inherit", textDecoration: "none" }}>
                <div className={styles.cellTitle} style={{ wordBreak: "break-all" }}>{u.email}</div>
                <div className={styles.mono} style={{ letterSpacing: 0, fontSize: 12 }}>{u.tenant_id}</div>
                <div>{u.disabled_at ? <StatusBadge status="disabled" tone="danger" /> : <StatusBadge status={u.email_verified ? "verified" : "unverified"} tone={u.email_verified ? "success" : "warning"} />}</div>
                <div className={styles.cellSub}>{u.last_login_at ? fmtDate(u.last_login_at) : "Never"}</div>
              </Link>
            ))}
          </div>
        </div>
      ) : (
        <>
          <div className={styles.table} style={{ "--cols": "2fr 2fr 1fr 1.2fr", "--min-w": "640px" }}>
            <div className={styles.thead}><div>Business</div><div>Tenant ID</div><div>Status</div><div>Created</div></div>
            <div className={styles.tbody}>
              {!tenants && !error && <SkeletonRows rows={3} cols={4} />}
              {tenants?.length === 0 && <div className={styles.empty}>No businesses match.</div>}
              {tenants?.map((t) => (
                <Link key={t.id} href={`/admin/tenants/${t.id}`} className={styles.trow} style={{ color: "inherit", textDecoration: "none" }}>
                  <div className={styles.cellTitle}>{t.name}</div>
                  <div className={styles.mono} style={{ letterSpacing: 0, fontSize: 12 }}>{t.id}</div>
                  <div><StatusBadge status={t.status} tone={t.status === "suspended" ? "danger" : "success"} /></div>
                  <div className={styles.cellSub}>{fmtDate(t.created_at)}</div>
                </Link>
              ))}
            </div>
          </div>
          {cursor && <button className="btn" style={{ marginTop: 12 }} onClick={() => load(cursor)}>Load more</button>}
        </>
      )}
      <ErrorText error={error} />
    </section>
  );
}
