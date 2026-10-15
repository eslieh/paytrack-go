"use client";

import Link from "next/link";
import { useState } from "react";
import styles from "./page.module.css";
import { getBaseUrl } from "@/lib/api";

const API = getBaseUrl();
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
            Collect straight into your bank account, PayBill or Till. No Daraja
            app needed: send an STK push with one API call and get signed
            webhooks when customers pay. Track every payment and
            delivery from a single dashboard.
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
              <CheckCircle2 size={18} /> STK Push into your bank account, PayBill or Till
            </li>
            <li>
              <CheckCircle2 size={18} /> Signed webhooks with automatic retries
              and redelivery
            </li>
            <li>
              <CheckCircle2 size={18} /> Idempotent requests, so retries never
              double‑charge
            </li>
          </ul>
        </div>

        <div className={styles.preview} aria-hidden>
          <div className={styles.previewHeader}>
            <span className={styles.badge}>Payment</span>
            <div className={styles.previewActions}>
              <span>STK Push</span>
              <button className={styles.smallBtn}>View dashboard</button>
            </div>
          </div>
          <div className={styles.previewBody}>
            <div className={styles.detailCard}>
              <div className={styles.detailRow}>
                <span>Amount</span>
                <strong>
                  <span className={styles.mono}>KES 250.00</span>
                </strong>
              </div>
              <div className={styles.detailRow}>
                <span>Status</span>
                <strong>
                  <span className={styles.mono}>succeeded</span>
                </strong>
              </div>
            </div>
            <div className={styles.historyCard}>
              <h4>Recent payments</h4>
              <div className={styles.historyItem}>
                <span className={styles.mono}>SUCCEEDED</span> • ORDER123 •{" "}
                <span className={styles.mono}>KES 250.00</span>
              </div>
              <div className={styles.historyItem}>
                <span className={styles.mono}>PENDING</span> • ORDER124 •{" "}
                <span className={styles.mono}>KES 1,000.00</span>
              </div>
              <div className={styles.historyItem}>
                <span className={styles.mono}>CANCELLED</span> • ORDER125
              </div>
            </div>
            <div className={styles.itemsCard}>
              <h4>Configuration</h4>
              <div className={styles.itemRow}>
                <span>Webhook URL</span>
                <strong>https://your.app/hooks/paytrack</strong>
              </div>
              <div className={styles.itemRow}>
                <span>Channel</span>
                <strong>Equity Bank · 0123456789</strong>
              </div>
            </div>
          </div>
        </div>
      </main>

      <section id="features" className={styles.features}>
        <div className={styles.feature}>
          <Zap />
          <h3>Blazing fast</h3>
          <p>Event-driven pipeline with status polling as a fallback.</p>
        </div>
        <div className={styles.feature}>
          <Webhook />
          <h3>Webhooks</h3>
          <p>HMAC-signed events, ~16h of retries and one-click redelivery.</p>
        </div>
        <div className={styles.feature}>
          <Shield />
          <h3>Secure</h3>
          <p>No Daraja credentials to manage; revocable API keys.</p>
        </div>
        <div className={styles.feature}>
          <Code2 />
          <h3>Clean API</h3>
          <p>REST + OpenAPI spec, idempotency keys and cursor pagination.</p>
        </div>
      </section>

      <section id="code" className={styles.codeBlock}>
        <div className={styles.codeHeader}>Charge with STK Push</div>
        <pre>
          {`curl -X POST ${API}/v1/payments \\
  -H "Authorization: Bearer ptk_***" \\
  -H "Idempotency-Key: order-123" \\
  -H "Content-Type: application/json" \\
  -d '{
    "channel_id": "chn_***",
    "amount": 250,
    "phone": "0712345678"
  }'`}
        </pre>
        <div className={styles.codeHeader}>Check payment status</div>
        <pre>
          {`curl ${API}/v1/payments/pay_*** \\
  -H "Authorization: Bearer ptk_***"`}
        </pre>
        <div className={styles.codeHeader}>Receive the outcome by webhook</div>
        <pre>
          {`POST https://your.app/hooks/paytrack
PayTrack-Event-Type: payment.succeeded
PayTrack-Signature: t=1760512345,v1=5f2b…

{ "id": "evt_***", "type": "payment.succeeded",
  "data": { "id": "pay_***", "status": "succeeded", "amount": 250,
            "mpesa_receipt_number": "TJK3XXXXXX", ... } }`}
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
              <CheckCircle2 size={18} /> Funds go straight to your bank, PayBill or Till
            </li>
            <li>
              <CheckCircle2 size={18} /> Standard M-Pesa charges apply
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
