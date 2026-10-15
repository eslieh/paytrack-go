"use client";

import Link from "next/link";
import { useState } from "react";
import styles from "./page.module.css";
import {
  ArrowRight,
  CheckCircle2,
  Code2,
  Shield,
  Zap,
  Webhook,
} from "lucide-react";

export default function Home() {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className={styles.landing}>
      <header className={styles.navbar}>
        <div className={styles.brand}>Paytrack</div>
        <nav className={styles.navlinks}>
          <Link href="#features">Features</Link>
          <Link href="#pricing">Pricing</Link>
          <Link href="/auth" className={styles.linkMuted}>
            Sign in
          </Link>
          <Link href="/auth/register" className={styles.signup}>
            Get started
          </Link>
        </nav>
        <button
          className={styles.menuBtn}
          aria-haspopup="true"
          aria-expanded={menuOpen}
          aria-controls="site-menu"
          onClick={() => setMenuOpen((v) => !v)}
        >
          Menu
        </button>
        {menuOpen && (
          <div id="site-menu" role="menu" className={styles.menu}>
            <Link
              role="menuitem"
              href="#features"
              className={styles.menuItem}
              onClick={() => setMenuOpen(false)}
            >
              Features
            </Link>
            <Link
              role="menuitem"
              href="#pricing"
              className={styles.menuItem}
              onClick={() => setMenuOpen(false)}
            >
              Pricing
            </Link>
            <Link
              role="menuitem"
              href="/auth"
              className={styles.menuItem}
              onClick={() => setMenuOpen(false)}
            >
              Sign in
            </Link>
            <Link
              role="menuitem"
              href="/auth/register"
              className={`${styles.menuItem} ${styles.menuPrimary}`}
              onClick={() => setMenuOpen(false)}
            >
              Get started
            </Link>
          </div>
        )}
      </header>

      <main className={styles.heroSection}>
        <div className={styles.heroCopy}>
          <h1>M‑Pesa integration, made effortless for developers</h1>
          <p>
            Ship payments fast with clean APIs, signed webhooks, token‑scoped
            keys and a live wallet you can trust. Configure callbacks, track
            transactions and automate payouts — all in one place.
          </p>
          <div className={styles.ctaRow}>
            <Link href="/auth/register" className={styles.ctaPrimary}>
              Start building
              <ArrowRight size={18} />
            </Link>
            <a href="#code" className={styles.ctaGhost}>
              See the code
            </a>
          </div>

          <ul className={styles.points}>
            <li>
              <CheckCircle2 size={18} /> STK Push, C2B, B2C & B2B built‑ins 
            </li>
            <li>
              <CheckCircle2 size={18} /> Signed webhooks, idempotency and replay
              protection
            </li>
            <li>
              <CheckCircle2 size={18} /> Wallet, transactions and scheduled
              auto‑payouts
            </li>
          </ul>
        </div>

        <div className={styles.preview} aria-hidden>
          <div className={styles.previewHeader}>
            <span className={styles.badge}>Wallet</span>
            <div className={styles.previewActions}>
              <span>Tenant Wallet</span>
              <button className={styles.smallBtn}>View dashboard</button>
            </div>
          </div>
          <div className={styles.previewBody}>
            <div className={styles.detailCard}>
              <div className={styles.detailRow}>
                <span>Balance</span>
                <strong>
                  <span className={styles.mono}>KES 0.00</span>
                </strong>
              </div>
              <div className={styles.detailRow}>
                <span>Account</span>
                <strong>
                  <span className={styles.mono}>•••• 1234</span>
                </strong>
              </div>
              {/* <a className={styles.pdfBtn} href="/dashboard/wallet">Open wallet</a> */}
            </div>
            <div className={styles.historyCard}>
              <h4>Recent transactions</h4>
              <div className={styles.historyItem}>
                <span className={styles.mono}>CREDIT</span> • STK Push •{" "}
                <span className={styles.mono}>KES 250.00</span>
              </div>
              <div className={styles.historyItem}>
                <span className={styles.mono}>DEBIT</span> • Payout •{" "}
                <span className={styles.mono}>KES 1,000.00</span>
              </div>
              <div className={styles.historyItem}>
                <span className={styles.mono}>GATEWAY</span> • M‑Pesa
              </div>
            </div>
            <div className={styles.itemsCard}>
              <h4>Configuration</h4>
              <div className={styles.itemRow}>
                <span>Callback URL</span>
                <strong>https://your.app/webhooks/mpesa</strong>
              </div>
              <div className={styles.itemRow}>
                <span>Auto payout</span>
                <strong>Disabled</strong>
              </div>
              {/* <a className={styles.pdfBtn} href="/dashboard/api-keys">Manage API keys</a> */}
            </div>
          </div>
        </div>
      </main>

      <section id="features" className={styles.features}>
        <div className={styles.feature}>
          <Zap />
          <h3>Blazing fast</h3>
          <p>Low latency callbacks and retries built in.</p>
        </div>
        <div className={styles.feature}>
          <Webhook />
          <h3>Webhooks</h3>
          <p>Signed payloads, retries and replay protection.</p>
        </div>
        <div className={styles.feature}>
          <Shield />
          <h3>Secure</h3>
          <p>Token-scoped keys and IP allow-listing.</p>
        </div>
        <div className={styles.feature}>
          <Code2 />
          <h3>SDKs</h3>
          <p>Type-safe SDKs and Postman collection.</p>
        </div>
      </section>

      <section id="code" className={styles.codeBlock}>
        <div className={styles.codeHeader}>Charge with STK Push</div>
        <pre>
          {`curl -X POST https://pay.geninworld.com/api/payment_request \\\n+  -H "Authorization: Bearer sk_test_***" \\\n+  -H "Content-Type: application/json" \\\n+  -d '{
            "amount": "15.00",
            "currency": "KES",
            "request_ref": "unique_request_reference",
            "mpesa_number": "254700000000"
          }'`}
        </pre>
        <div className={styles.codeHeader}>Disburse to B2B/B2C Account</div>
        <pre>
          {`curl -X POST https://pay.geninworld.com/api/disburse_request \\\n+  -H "Authorization: Bearer sk_test_***" \\\n+  -H "Content-Type: application/json" \\\n+  -d '{
            "amount": "12",
            // "mpesa_number": "254700000000", optional to b2b account
            "request_ref": "abcdefesscndjkcd",
            "b2b_account": {
                "paybill_number": "1234",
                "account_number": "456"
            } // optional to b2c account
          }'`}
        </pre>
        <div className={styles.codeHeader}>Query collection status</div>
        <pre>
          {`curl -X GET https://pay.geninworld.com/api/payment/<string:request_ref>/status \\\n+  -H "Authorization: Bearer sk_test_***"`}
        </pre>
        <div className={styles.codeHeader}>Query Disbursement Status</div>
        <pre>
          {`curl -X GET https://pay.geninworld.com/api/disburse/<string:disbursement_identifier>/status \\\n+  -H "Authorization: Bearer sk_test_***"`}
        </pre>
      </section>

      <section id="pricing" className={styles.pricing}>
        <div className={styles.priceCard}>
          <div className={styles.priceHeader}>Simple pricing</div>
          <div className={styles.priceValue}>1.5%</div>
          <div className={styles.priceNote}>per successful collection</div>
          <ul className={styles.priceList}>
            <li>
              <CheckCircle2 size={18} /> No monthly fees
            </li>
            <li>
              <CheckCircle2 size={18} /> Free test mode
            </li>
            <li>
              <CheckCircle2 size={18} /> Instant settlements & payouts
            </li>
            <li>
              <CheckCircle2 size={18} /> Normal M-pesa b2c and b2b changes apply
            </li>
          </ul>
          <Link href="/auth/register" className={styles.ctaPrimary}>
            Get started
            <ArrowRight size={18} />
          </Link>
        </div>
      </section>

      <footer className={styles.footer}>
        <span>© {new Date().getFullYear()} Paytrack from Geninworld Limited</span>
        <div className={styles.footerLinks}>
          <a href="#pricing">Pricing</a>
          <a href="#features">Features</a>
          <Link href="/auth">Sign in</Link>
        </div>
      </footer>
    </div>
  );
}
