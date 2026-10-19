"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import {
  ArrowRight,
  BookOpen,
  Check,
  CircleOff,
  Code2,
  KeyRound,
  LayoutDashboard,
  Repeat,
  ShieldCheck,
  Waypoints,
  Webhook,
  Zap,
} from "lucide-react";
import styles from "./page.module.css";
import { getBaseUrl } from "@/lib/api";
import { highlight } from "@/app/docs/code";
import TestDrive from "@/components/TestDrive.jsx";
import BuiltBy from "@/components/BuiltBy.jsx";

const API = getBaseUrl();

const NAV = [
  { href: "#how", label: "How it works" },
  { href: "#features", label: "Features" },
  { href: "/docs", label: "Docs" },
];

const STATS = [
  { value: 1, label: "API call to charge a customer" },
  { value: 0, label: "shillings held by us. Ever." },
  { value: 16, prefix: "~", suffix: "h", label: "of automatic webhook retries" },
  { value: 3, label: "destinations: bank, PayBill, Till" },
];

// useReveal adds .in to every [data-reveal] element under root as it scrolls
// into view. Content is only hidden once this runs (root gets .revealReady), so
// nothing stays invisible without JavaScript or under reduced motion.
function useReveal(root, readyClass) {
  useEffect(() => {
    const el = root.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const items = el.querySelectorAll("[data-reveal]");
    el.classList.add(readyClass);
    const io = new IntersectionObserver((entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue;
        e.target.classList.add("in");
        io.unobserve(e.target);
      }
    }, { rootMargin: "0px 0px -10% 0px", threshold: 0.1 });
    items.forEach((n) => io.observe(n));
    return () => io.disconnect();
  }, [root, readyClass]);
}

// CountUp ticks from 0 to `to` once it scrolls into view.
function CountUp({ to, prefix = "", suffix = "" }) {
  const ref = useRef(null);
  const [n, setN] = useState(to);
  useEffect(() => {
    const el = ref.current;
    if (!el || to === 0 || typeof IntersectionObserver === "undefined") return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    setN(0);
    let raf;
    const io = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) return;
      io.disconnect();
      const start = performance.now();
      const tick = (t) => {
        const p = Math.min(1, (t - start) / 900);
        setN(Math.round(to * (1 - Math.pow(1 - p, 3)))); // ease-out cubic
        if (p < 1) raf = requestAnimationFrame(tick);
      };
      raf = requestAnimationFrame(tick);
    }, { threshold: 0.6 });
    io.observe(el);
    return () => { io.disconnect(); cancelAnimationFrame(raf); };
  }, [to]);
  return <span ref={ref}>{prefix}{n}{suffix}</span>;
}

const SOLVES = [
  {
    problem: "Multi-step auth and token juggling",
    how: "One API key, sent as a bearer token",
    win: "No OAuth tokens that expire every hour, no password-and-timestamp signatures to generate.",
  },
  {
    problem: "Getting money to the right account",
    how: "Direct-to-destination routing",
    win: "Funds land straight in your bank account, PayBill or Till. Paytrack never holds them.",
  },
  {
    problem: "Callbacks that go missing",
    how: "HMAC-signed webhooks, ~16h of retries, status reconciliation",
    win: "Verify every event cryptographically, replay any delivery in one click, and catch outcomes even when a callback is lost.",
  },
  {
    problem: "Double charges on flaky networks",
    how: "Idempotency-Key on every payment",
    win: "Retry as often as you like. Same key, same payment, one prompt to the customer.",
  },
];

const STEPS = [
  {
    n: "01",
    title: "Add where the money goes",
    text: "Register a bank account, PayBill or Till once. You get a channel ID back.",
    lang: "sh",
    code: `curl -X POST ${API}/v1/channels \\
  -H "Authorization: Bearer $PAYTRACK_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "Main account",
    "type": "bank",
    "bank_code": "247247",
    "account_number": "0123456789"
  }'`,
  },
  {
    n: "02",
    title: "Charge the customer",
    text: "One call sends the M-Pesa prompt to their phone. Use your order ID as the idempotency key.",
    lang: "sh",
    code: `curl -X POST ${API}/v1/payments \\
  -H "Authorization: Bearer $PAYTRACK_API_KEY" \\
  -H "Idempotency-Key: order-8891" \\
  -H "Content-Type: application/json" \\
  -d '{
    "channel_id": "chn_01k7x4e2p3q4r5s6t7v8w9x0yz",
    "amount": 250,
    "phone": "0712345678"
  }'`,
  },
  {
    n: "03",
    title: "Get told when it lands",
    text: "Verify the signature, dedupe on the event ID, fulfil the order. That's the whole integration.",
    lang: "js",
    code: `app.post("/hooks/paytrack", express.raw({ type: "application/json" }), (req, res) => {
  const { t, v1 } = Object.fromEntries(
    req.get("PayTrack-Signature").split(",").map((p) => p.split("="))
  );
  const expected = crypto.createHmac("sha256", process.env.PAYTRACK_SIGNING_SECRET)
    .update(\`\${t}.\${req.body}\`).digest("hex");
  if (expected !== v1) return res.sendStatus(400); // docs: timing-safe compare + replay check

  const event = JSON.parse(req.body);
  if (event.type === "payment.succeeded") {
    markPaid(event.data.metadata.order_id, event.data.mpesa_receipt_number);
  }
  res.sendStatus(200);
});`,
  },
];

const EVENT = `POST /hooks/paytrack HTTP/1.1
PayTrack-Event-Type: payment.succeeded
PayTrack-Signature: t=1760512345,v1=5f2b8a9c…

{
  "id": "evt_01k7x5h8a9b0c1d2e3f4g5h6jk",
  "type": "payment.succeeded",
  "data": {
    "id": "pay_01k7x5g3r4s5t6v7w8x9y0z1ab",
    "status": "succeeded",
    "amount": 250,
    "currency": "KES",
    "mpesa_receipt_number": "TJK3Q8XD2F",
    "metadata": { "order_id": "8891" }
  }
}`;

const FEATURES = [
  { icon: Zap, title: "Event-driven core", text: "Callbacks first, background status queries as backup. Payments reach a final state even when the network doesn't cooperate." },
  { icon: ShieldCheck, title: "Signed webhooks", text: "HMAC-SHA256 over timestamp + body. Reject replays older than five minutes and trust every payload you accept." },
  { icon: Repeat, title: "Idempotency built in", text: "Send an Idempotency-Key and retries replay the original payment instead of prompting the customer twice." },
  { icon: CircleOff, title: "Non-custodial", text: "No balances, no float, no payout queue. Money moves from your customer straight to your account." },
  { icon: LayoutDashboard, title: "One console", text: "Payments, channels, API keys and webhook delivery logs, with one-click redelivery, in a single dashboard." },
  { icon: Code2, title: "Boring, good API", text: "REST + JSON, an OpenAPI 3.1 spec, cursor pagination, stable error codes and a request ID on every response." },
];

function Code({ code, lang, title }) {
  return (
    <div className={styles.code}>
      {title && <div className={styles.codeTitle}>{title}</div>}
      <pre dangerouslySetInnerHTML={{ __html: highlight(code, lang) }} />
    </div>
  );
}

export default function Home() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [step, setStep] = useState(0);
  const root = useRef(null);
  useReveal(root, styles.revealReady);

  return (
    <div className={styles.landing} ref={root}>
      {/* ------------------------------------------------ nav */}
      <header className={styles.navbar}>
        <Link href="/" className={styles.brand}>Paytrack</Link>
        <nav className={styles.navlinks}>
          {NAV.map((n) => <Link key={n.href} href={n.href}>{n.label}</Link>)}
          <Link href="/auth" className={styles.linkMuted}>Sign in</Link>
          <Link href="/auth/register" className={styles.signup}>Get API key</Link>
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
            {[...NAV, { href: "/auth", label: "Sign in" }].map((n) => (
              <Link key={n.href} role="menuitem" href={n.href} className={styles.menuItem} onClick={() => setMenuOpen(false)}>{n.label}</Link>
            ))}
            <Link role="menuitem" href="/auth/register" className={`${styles.menuItem} ${styles.menuPrimary}`} onClick={() => setMenuOpen(false)}>
              Get API key
            </Link>
          </div>
        )}
      </header>

      <main>
        {/* ------------------------------------------------ hero */}
        <section className={styles.heroSection}>
          <div className={styles.heroCopy}>
            <h1>
              Instant payment collection <span className={styles.accent}>&amp; routing</span>, built for developers.
            </h1>
            <p className={styles.lede}>
              Route customer payments straight into your bank account, PayBill or Till with one API call.
              No Daraja app to register, no tokens to refresh, and HMAC-signed webhooks the moment the money lands.
            </p>
            <div className={styles.ctaRow}>
              <Link href="/auth/register" className={styles.ctaPrimary}>
                Get your API key <ArrowRight size={18} />
              </Link>
              <Link href="/docs#quickstart" className={styles.ctaGhost}>
                <BookOpen size={18} /> Read the quickstart
              </Link>
            </div>
            <ul className={styles.points}>
              <li><Check size={16} /> Live in an afternoon</li>
              <li><Check size={16} /> Your money, your account</li>
              <li><Check size={16} /> Retries that never double-charge</li>
            </ul>
          </div>
          <TestDrive />
        </section>

        {/* ------------------------------------------------ stats */}
        <section className={styles.stats} aria-label="At a glance">
          {STATS.map((s) => (
            <div key={s.label} className={styles.stat} data-reveal>
              <div className={styles.statValue}><CountUp to={s.value} prefix={s.prefix} suffix={s.suffix} /></div>
              <div className={styles.statLabel}>{s.label}</div>
            </div>
          ))}
        </section>

        {/* ------------------------------------------------ what it solves */}
        <section className={styles.section}>
          <div className={styles.sectionHead} data-reveal>
            <div className={styles.kicker}>Why Paytrack</div>
            <h2>The annoying parts of payments, already handled</h2>
            <p>You write the business logic. We deal with tokens, prompts, callbacks, retries and reconciliation.</p>
          </div>
          <div className={styles.solveTable} role="table" data-reveal>
            <div className={`${styles.solveRow} ${styles.solveHead}`} role="row">
              <span role="columnheader">The problem</span><span role="columnheader">How Paytrack handles it</span><span role="columnheader">What you get</span>
            </div>
            {SOLVES.map((r) => (
              <div key={r.problem} className={styles.solveRow} role="row">
                <span role="cell" className={styles.problem}>{r.problem}</span>
                <span role="cell" className={styles.how}>{r.how}</span>
                <span role="cell">{r.win}</span>
              </div>
            ))}
          </div>
        </section>

        {/* ------------------------------------------------ how it works */}
        <section id="how" className={styles.section}>
          <div className={styles.sectionHead} data-reveal>
            <div className={styles.kicker}>How it works</div>
            <h2>Three steps. Most of it is copy-paste.</h2>
          </div>
          <div className={styles.how3} data-reveal>
            <ol className={styles.steps}>
              {STEPS.map((s, i) => (
                <li key={s.n}>
                  <button className={`${styles.stepBtn} ${i === step ? styles.stepOn : ""}`} onClick={() => setStep(i)} aria-pressed={i === step}>
                    <span className={styles.stepN}>{s.n}</span>
                    <span>
                      <strong>{s.title}</strong>
                      <span className={styles.stepText}>{s.text}</span>
                    </span>
                  </button>
                </li>
              ))}
            </ol>
            <div className={styles.stepCode}>
              <Code key={step} code={STEPS[step].code} lang={STEPS[step].lang} title={STEPS[step].lang === "js" ? "server.js" : "terminal"} />
              {step === 2 && <Code code={EVENT} lang="json" title="what your endpoint receives" />}
            </div>
          </div>
        </section>

        {/* ------------------------------------------------ features */}
        <section id="features" className={styles.section}>
          <div className={styles.sectionHead} data-reveal>
            <div className={styles.kicker}>Features</div>
            <h2>Built like infrastructure, not a weekend hack</h2>
            <p>Everything you&apos;d build yourself if you had a spare quarter.</p>
          </div>
          <div className={styles.features}>
            {FEATURES.map(({ icon: Icon, title, text }) => (
              <div key={title} className={styles.feature} data-reveal>
                <Icon />
                <h3>{title}</h3>
                <p>{text}</p>
              </div>
            ))}
          </div>
        </section>

        {/* ------------------------------------------------ destinations */}
        <section className={styles.section}>
          <div className={styles.dest} data-reveal>
            <div>
              <div className={styles.kicker}>Routing</div>
              <h2>Point it at any account you already have</h2>
              <p>Every channel is just a destination. Charge through any of them with the same call.</p>
            </div>
            <div className={styles.destCards}>
              <div className={`${styles.destCard} ${styles.destBank}`}><Waypoints size={18} /><strong>Bank account</strong><span>Equity, KCB, Co-op and more, via the bank&apos;s PayBill</span></div>
              <div className={`${styles.destCard} ${styles.destPaybill}`}><KeyRound size={18} /><strong>PayBill</strong><span>A default account, or one per payment</span></div>
              <div className={`${styles.destCard} ${styles.destTill}`}><Webhook size={18} /><strong>Till</strong><span>Buy Goods, nothing else to configure</span></div>
            </div>
          </div>
        </section>

        {/* ------------------------------------------------ final CTA */}
        <section className={styles.final} data-reveal>
          <h2>Your first STK push is about five minutes away.</h2>
          <p>Create an account, grab an API key, paste the curl. If it doesn&apos;t feel easier than what you have now, close the tab.</p>
          <div className={styles.ctaRow}>
            <Link href="/auth/register" className={styles.ctaPrimary}>Get your API key <ArrowRight size={18} /></Link>
            <Link href="/docs" className={styles.ctaGhost}>Browse the API reference</Link>
          </div>
        </section>
      </main>

      <footer className={styles.footer}>
        <BuiltBy />
        <div className={styles.footerLinks}>
          <Link href="/docs">Docs</Link>
          <Link href="/terms">Terms of Service</Link>
          <Link href="/privacy">Privacy Policy</Link>
          <Link href="/disclaimer">Disclaimer</Link>
          <Link href="/auth">Sign in</Link>
        </div>
      </footer>
    </div>
  );
}
