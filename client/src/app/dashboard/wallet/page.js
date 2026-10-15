"use client";

import { useEffect, useMemo, useState } from "react";
import Tabs from "@/components/Tabs.jsx";
import { apiFetch } from "@/lib/api";
import styles from "@/app/dashboard/styles.module.css";

const fmtKES = (n) => `KES ${Number(n ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

export default function WalletPage() {
  const [wallet, setWallet] = useState(null);
  const [txns, setTxns] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [cursor, setCursor] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  async function fetchPage(next) {
    const q = next ? `?cursor=${encodeURIComponent(next)}` : "";
    const res = await apiFetch(`/tenants/dashboard/wallet${q}`, { method: "GET" });
    return res;
  }

  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const res = await fetchPage(null);
        if (!mounted) return;
        setWallet(res.wallet);
        setTxns(res.transactions || []);
        setCursor(res.pagination?.next_cursor || null);
        setHasMore(!!res.pagination?.has_more);
      } catch (e) {
        if (mounted) setError(e.message || "Failed to load wallet");
      } finally {
        if (mounted) setLoading(false);
      }
    })();
    return () => { mounted = false; };
  }, []);

  async function loadMore() {
    if (!hasMore || loadingMore) return;
    setLoadingMore(true);
    try {
      const res = await fetchPage(cursor);
      setTxns((prev) => prev.concat(res.transactions || []));
      setCursor(res.pagination?.next_cursor || null);
      setHasMore(!!res.pagination?.has_more);
    } catch (e) {
      setError(e.message || "Failed to load more");
    } finally {
      setLoadingMore(false);
    }
  }

  const maskedAccount = useMemo(() => {
    const n = wallet?.account_no;
    return n ? String(n).replace(/.(?=.{4})/g, "•") : "••••";
  }, [wallet]);

  return (
    <div className={styles.shell}>
      <Tabs
        activeHref="/dashboard/wallet"
        items={[
          { label: "Home", href: "/dashboard" },
          { label: "Wallet", href: "/dashboard/wallet" },
          { label: "API Keys", href: "/dashboard/api-keys" },
        ]}
        brand="Paytrack"
      />
      <main className={styles.main}>
        {/* Wallet card + summary */}
        <section>
          <div className={styles.walletCard}>
            <div className={styles.glow1} />
            <div className={styles.glow2} />
            <div className={styles.walletHeader}>
              <div className={styles.brandMark}>Paytrack Wallet</div>
              <div className={styles.chip}><div className={styles.chipBoxA} /><div className={styles.chipBoxB} /></div>
            </div>
            <div className={styles.walletBody}>
              <div className={styles.label}>Tenant Name</div>
              <div className={styles.holder}>{loading ? "Loading..." : wallet?.name || "—"}</div>
              <div className={styles.infoRow}>
                <div>
                  <div className={styles.subLabel}>Account</div>
                  <div className={styles.mono}>{maskedAccount}</div>
                </div>
                <div className={styles.amount}>
                  <div className={styles.subLabel}>Balance</div>
                  {loading ? "—" : fmtKES(wallet?.balance)}
                </div>
              </div>
            </div>
          </div>
          <div className={styles.summaryGrid}>
            <div className={styles.summaryCard}>
              <div className={styles.summaryLabel}>Credits</div>
              <div className={styles.summaryValue}>{loading ? "—" : fmtKES(wallet?.totals?.credit)}</div>
            </div>
            <div className={styles.summaryCard}>
              <div className={styles.summaryLabel}>Debits</div>
              <div className={styles.summaryValue}>{loading ? "—" : fmtKES(wallet?.totals?.debit)}</div>
            </div>
            <div className={styles.summaryCard}>
              <div className={styles.summaryLabel}>Gateway</div>
              <div className={styles.summaryValue}>M-Pesa</div>
            </div>
          </div>
        </section>

        {/* Transactions table */}
        <section className={styles.card}>
          <div style={{display: "flex", alignItems: "center", justifyContent: "space-between"}}>
            <div className={styles.h2}>Transactions</div>
            <div className={styles.muted}>Gateway: M-Pesa</div>
          </div>
          <div className={styles.table}>
            <div className={styles.thead}>
              <div>Description</div>
              <div>Type</div>
              <div>Amount</div>
              <div>Status</div>
              <div>Customer</div>
              <div>Date</div>
            </div>
            <div className={styles.tbody}>
              {loading && Array.from({ length: 5 }).map((_, i) => (<div key={i} className={styles.skelRow || styles.skeletonLine} style={{height: 40}} />))}
              {!loading && txns.map((t) => (
                <div key={t.id} className={styles.trow}>
                  <div>Txn {t.transaction_ref}</div>
                  <div><span className={t.type === "credit" ? styles.pill : styles.pill}>{t.type.toUpperCase()}</span></div>
                  <div className={t.type === "credit" ? styles.amountCredit : styles.amountDebit}>{fmtKES(t.amount)}</div>
                  <div><span className={styles.badgeSuccess}>{t.status}</span></div>
                  {(t.receiving_mpesa_number || t.b2b_account?.account_number || t.b2b_account?.paybill_number)?(
                    <div>
                      {t.receiving_mpesa_number ?
                        <div>
                          mpesa: {`${t.receiving_mpesa_number.slice(0, 4)}****${t.receiving_mpesa_number.slice(-4)}`}
                        </div>
                        :
                        <div>
                          b2b: account: {t.b2b_account?.account_number ? `${t.b2b_account.account_number.slice(0,2)}****${t.b2b_account.account_number.slice(-2)}` : ""}
                          {" "}
                          paybill: {t.b2b_account?.paybill_number ? `${t.b2b_account.paybill_number.slice(0,2)}****${t.b2b_account.paybill_number.slice(-2)}` : ""}
                        </div>
                      }
                    </div>
                  ) : <div>Mpesa: {t.account_no ? `${t.account_no.slice(0, 4)}****${t.account_no.slice(-4)}` : ""}</div>}
                  <div>{new Date(t.created_at).toLocaleString()}</div>
                </div>
              ))}
            </div>
          </div>
          <div className={styles.loadMoreRow}>
            {hasMore ? (
              <button className="btn btn-ghost" onClick={loadMore} disabled={loadingMore}>{loadingMore ? "Loading..." : "Load more"}</button>
            ) : (
              <span className={styles.muted}>No more transactions</span>
            )}
          </div>
          {error && (<div style={{color: "#b42318", fontSize: 13, marginTop: 8}}>{error}</div>)}
        </section>
      </main>
    </div>
  );
}


