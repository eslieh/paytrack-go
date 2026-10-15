"use client";

import Link from "next/link";
import { Fragment, useEffect, useMemo, useState } from "react";
import { ArrowRight, Check, Copy, Menu, Search, X } from "lucide-react";
import styles from "./docs.module.css";
import { getBaseUrl } from "@/lib/api";
import { DASHBOARD_URL, ENDPOINT_GROUPS, ERROR_CODES, ERROR_STATUSES, EX, OBJECTS, WEBHOOK_RETRY_SCHEDULE } from "./content";
import { LANGS, highlight, sample } from "./code";

const BASE = getBaseUrl();
const LANG_KEY = "paytrack_docs_lang";

// ------------------------------------------------------------------ navigation

const GUIDES = [
  { id: "introduction", label: "Introduction" },
  { id: "quickstart", label: "Quickstart" },
  { id: "authentication", label: "Authentication" },
  { id: "errors", label: "Errors" },
  { id: "pagination", label: "Pagination" },
  { id: "idempotency", label: "Idempotency" },
  { id: "phone-amounts", label: "Phones & amounts" },
  { id: "lifecycle", label: "Payment lifecycle" },
  { id: "routing", label: "Routing money" },
  { id: "webhooks", label: "Webhooks" },
  { id: "go-live", label: "Go-live checklist" },
];

const NAV = [
  { title: "Get started", items: GUIDES.slice(0, 3) },
  { title: "Core concepts", items: GUIDES.slice(3, 9) },
  { title: "Webhooks", items: GUIDES.slice(9) },
  { title: "Objects", items: OBJECTS.map((o) => ({ id: o.id, label: o.name.replace(/^The | object$/g, "") })) },
  ...ENDPOINT_GROUPS.map((g) => ({
    title: g.title,
    items: g.endpoints.map((e) => ({ id: e.id, label: e.title, method: e.method })),
  })),
];

// ------------------------------------------------------------------ small pieces

// Renders `code` spans and **bold** in content strings.
function Md({ children }) {
  const parts = String(children).split(/(`[^`]+`|\*\*[^*]+\*\*)/g);
  return parts.map((p, i) =>
    p.startsWith("`") ? <code key={i}>{p.slice(1, -1)}</code>
    : p.startsWith("**") ? <strong key={i}>{p.slice(2, -2)}</strong>
    : <Fragment key={i}>{p}</Fragment>,
  );
}

function CopyBtn({ text }) {
  const [done, setDone] = useState(false);
  return (
    <button
      className={styles.copyBtn}
      onClick={async () => {
        try { await navigator.clipboard.writeText(text); setDone(true); setTimeout(() => setDone(false), 1200); } catch {}
      }}
      aria-label="Copy code"
    >
      {done ? <Check size={14} /> : <Copy size={14} />}
      {done ? "Copied" : "Copy"}
    </button>
  );
}

function Code({ code, lang = "json", title, tabs }) {
  return (
    <div className={styles.code}>
      <div className={styles.codeHead}>
        {tabs || <span className={styles.codeTitle}>{title}</span>}
        <CopyBtn text={code} />
      </div>
      <pre dangerouslySetInnerHTML={{ __html: highlight(code, lang) }} />
    </div>
  );
}

function LangTabs({ lang, setLang }) {
  return (
    <div className={styles.langTabs} role="tablist">
      {LANGS.map((l) => (
        <button key={l.id} role="tab" aria-selected={lang === l.id} className={lang === l.id ? styles.langActive : ""} onClick={() => setLang(l.id)}>
          {l.label}
        </button>
      ))}
    </div>
  );
}

function RequestSample({ ep, lang, setLang, req, title }) {
  const hl = LANGS.find((l) => l.id === lang)?.hl;
  return (
    <Code
      code={sample(lang, BASE, ep, req)}
      lang={hl}
      tabs={
        <div className={styles.codeHeadLeft}>
          <span className={styles.codeTitle}>{title || <><Method m={ep.method} /> {ep.path}</>}</span>
          <LangTabs lang={lang} setLang={setLang} />
        </div>
      }
    />
  );
}

function Responses({ responses }) {
  const [i, setI] = useState(0);
  const r = responses[i];
  return (
    <Code
      code={r.body == null ? "// No content" : JSON.stringify(r.body, null, 2)}
      lang="json"
      tabs={
        <div className={styles.codeHeadLeft}>
          <span className={styles.codeTitle}>Response</span>
          <div className={styles.langTabs}>
            {responses.map((x, j) => (
              <button key={x.status} aria-selected={i === j} className={i === j ? styles.langActive : ""} onClick={() => setI(j)}>
                <span className={x.status.startsWith("2") ? styles.dotOk : styles.dotErr} />
                {x.status.split(" ")[0]}
              </button>
            ))}
          </div>
        </div>
      }
    />
  );
}

function Method({ m }) {
  return <span className={`${styles.method} ${styles[`m${m}`]}`}>{m === "DELETE" ? "DEL" : m}</span>;
}

function AuthBadge({ auth }) {
  if (auth === "none") return <span className={styles.authNone}>No auth</span>;
  if (auth === "session") return <span className={styles.authSession}>Dashboard session only</span>;
  return <span className={styles.authKey}>API key or session</span>;
}

function Params({ title, rows }) {
  if (!rows?.length) return null;
  return (
    <div className={styles.params}>
      <div className={styles.paramsTitle}>{title}</div>
      {rows.map(([name, type, required, desc]) => (
        <div key={name} className={styles.param}>
          <div className={styles.paramHead}>
            <code className={styles.paramName}>{name}</code>
            <span className={styles.paramType}>{type}</span>
            {required ? <span className={styles.required}>required</span> : <span className={styles.optional}>optional</span>}
          </div>
          <div className={styles.paramDesc}><Md>{desc}</Md></div>
        </div>
      ))}
    </div>
  );
}

function Fields({ rows }) {
  return (
    <div className={styles.params}>
      <div className={styles.paramsTitle}>Attributes</div>
      {rows.map(([name, type, desc]) => (
        <div key={name} className={styles.param}>
          <div className={styles.paramHead}>
            <code className={styles.paramName}>{name}</code>
            <span className={styles.paramType}>{type}</span>
          </div>
          <div className={styles.paramDesc}><Md>{desc}</Md></div>
        </div>
      ))}
    </div>
  );
}

function Section({ id, children, split }) {
  return (
    <section id={id} className={split ? styles.split : styles.section} data-section>
      {children}
    </section>
  );
}

function Callout({ tone = "info", children }) {
  return <div className={`${styles.callout} ${styles[tone]}`}>{children}</div>;
}

// ------------------------------------------------------------------ webhook verification samples

const VERIFY = {
  node: `import crypto from "node:crypto";
import express from "express";

const app = express();
const secret = process.env.PAYTRACK_SIGNING_SECRET; // whsec_...

// Use the RAW body: re-serialised JSON won't match the signature.
app.post("/hooks/paytrack", express.raw({ type: "application/json" }), (req, res) => {
  const header = req.get("PayTrack-Signature") || "";
  const { t, v1 } = Object.fromEntries(header.split(",").map((kv) => kv.split("=")));

  const expected = crypto.createHmac("sha256", secret).update(\`\${t}.\${req.body}\`).digest("hex");
  const valid = v1 && v1.length === expected.length &&
    crypto.timingSafeEqual(Buffer.from(v1), Buffer.from(expected));
  const fresh = Math.abs(Date.now() / 1000 - Number(t)) <= 300;
  if (!valid || !fresh) return res.sendStatus(400);

  const event = JSON.parse(req.body);
  if (alreadyProcessed(event.id)) return res.sendStatus(200); // dedupe

  switch (event.type) {
    case "payment.succeeded": markOrderPaid(event.data.metadata.order_id, event.data); break;
    case "payment.failed":
    case "payment.cancelled":
    case "payment.expired":   markOrderUnpaid(event.data.metadata.order_id, event.data); break;
  }
  res.sendStatus(200); // respond fast; do slow work in a queue
});`,
  python: `import hashlib, hmac, json, os, time
from flask import Flask, request, abort

app = Flask(__name__)
SECRET = os.environ["PAYTRACK_SIGNING_SECRET"].encode()  # whsec_...

@app.post("/hooks/paytrack")
def paytrack_webhook():
    raw = request.get_data()  # the RAW body, byte for byte
    parts = dict(p.split("=", 1) for p in request.headers.get("PayTrack-Signature", "").split(",") if "=" in p)
    t, v1 = parts.get("t", "0"), parts.get("v1", "")

    expected = hmac.new(SECRET, f"{t}.".encode() + raw, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(expected, v1) or abs(time.time() - int(t)) > 300:
        abort(400)

    event = json.loads(raw)
    if already_processed(event["id"]):  # dedupe on event id
        return "", 200

    if event["type"] == "payment.succeeded":
        mark_order_paid(event["data"]["metadata"].get("order_id"), event["data"])
    else:  # payment.failed | payment.cancelled | payment.expired
        mark_order_unpaid(event["data"]["metadata"].get("order_id"), event["data"])
    return "", 200`,
  php: `<?php
$secret = getenv("PAYTRACK_SIGNING_SECRET"); // whsec_...
$raw = file_get_contents("php://input");      // the RAW body

parse_str(str_replace(",", "&", $_SERVER["HTTP_PAYTRACK_SIGNATURE"] ?? ""), $sig);
$expected = hash_hmac("sha256", ($sig["t"] ?? "") . "." . $raw, $secret);

if (!hash_equals($expected, $sig["v1"] ?? "") || abs(time() - (int)($sig["t"] ?? 0)) > 300) {
    http_response_code(400);
    exit;
}

$event = json_decode($raw, true);
if (already_processed($event["id"])) { http_response_code(200); exit; } // dedupe

if ($event["type"] === "payment.succeeded") {
    mark_order_paid($event["data"]["metadata"]["order_id"] ?? null, $event["data"]);
} else {
    mark_order_unpaid($event["data"]["metadata"]["order_id"] ?? null, $event["data"]);
}
http_response_code(200);`,
  go: `// Go (net/http)
func paytrackWebhook(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // the RAW body
	var t, v1 string
	for _, p := range strings.Split(r.Header.Get("PayTrack-Signature"), ",") {
		if k, v, ok := strings.Cut(p, "="); ok && k == "t" {
			t = v
		} else if ok && k == "v1" {
			v1 = v
		}
	}
	mac := hmac.New(sha256.New, []byte(os.Getenv("PAYTRACK_SIGNING_SECRET")))
	mac.Write([]byte(t + "."))
	mac.Write(raw)
	ts, _ := strconv.ParseInt(t, 10, 64)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(v1)) || time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	var event struct {
		ID   string          \`json:"id"\`
		Type string          \`json:"type"\`
		Data json.RawMessage \`json:"data"\`
	}
	json.Unmarshal(raw, &event)
	// dedupe on event.ID, then handle event.Type
	w.WriteHeader(http.StatusOK)
}`,
};
const VERIFY_TABS = [
  { id: "node", label: "Node.js", hl: "js" },
  { id: "python", label: "Python", hl: "py" },
  { id: "php", label: "PHP", hl: "php" },
  { id: "go", label: "Go", hl: "go" },
];

// Follows the page language when it has a matching tab; Go is local to this block.
function VerifySample({ lang }) {
  const [own, setOwn] = useState(null);
  useEffect(() => setOwn(null), [lang]);
  const tab = VERIFY_TABS.find((t) => t.id === (own || lang)) || VERIFY_TABS[0];
  return (
    <Code
      code={VERIFY[tab.id]}
      lang={tab.hl}
      tabs={
        <div className={styles.codeHeadLeft}>
          <span className={styles.codeTitle}>Verify &amp; handle</span>
          <div className={styles.langTabs}>
            {VERIFY_TABS.map((t) => (
              <button key={t.id} aria-selected={tab.id === t.id} className={tab.id === t.id ? styles.langActive : ""} onClick={() => setOwn(t.id)}>{t.label}</button>
            ))}
          </div>
        </div>
      }
    />
  );
}

// ------------------------------------------------------------------ page

export default function DocsPage() {
  const [lang, setLangState] = useState("curl");
  const [active, setActive] = useState("introduction");
  const [filter, setFilter] = useState("");
  const [navOpen, setNavOpen] = useState(false);

  useEffect(() => {
    try { const l = localStorage.getItem(LANG_KEY); if (l && LANGS.some((x) => x.id === l)) setLangState(l); } catch {}
  }, []);
  const setLang = (l) => { setLangState(l); try { localStorage.setItem(LANG_KEY, l); } catch {} };

  // Scroll-spy: highlight the section nearest the top of the viewport.
  useEffect(() => {
    const hash = decodeURIComponent(window.location.hash.slice(1));
    if (hash && document.getElementById(hash)) setActive(hash);
    const els = [...document.querySelectorAll("[data-section]")];
    const obs = new IntersectionObserver(
      (entries) => {
        const vis = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (vis[0]) setActive(vis[0].target.id);
      },
      { rootMargin: "-80px 0px -65% 0px" },
    );
    els.forEach((el) => obs.observe(el));
    return () => obs.disconnect();
  }, []);

  useEffect(() => {
    const el = document.querySelector(`[data-nav="${active}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const nav = useMemo(() => {
    const f = filter.trim().toLowerCase();
    if (!f) return NAV;
    return NAV.map((g) => ({ ...g, items: g.items.filter((i) => `${i.label} ${i.method || ""} ${g.title}`.toLowerCase().includes(f)) })).filter((g) => g.items.length);
  }, [filter]);

  return (
    <div className={styles.docs}>
      <header className={styles.topbar}>
        <button className={styles.menuToggle} onClick={() => setNavOpen((v) => !v)} aria-label="Toggle navigation">
          {navOpen ? <X size={18} /> : <Menu size={18} />}
        </button>
        <Link href="/" className={styles.brand}>Paytrack <span>Docs</span></Link>
        <span className={styles.version}>API v1</span>
        <div className={styles.topLinks}>
          <Link href="/">Home</Link>
          <Link href="/dashboard" className={styles.topCta}>Dashboard <ArrowRight size={14} /></Link>
        </div>
      </header>

      <aside className={`${styles.sidebar} ${navOpen ? styles.sidebarOpen : ""}`}>
        <label className={styles.search}>
          <Search size={14} />
          <input placeholder="Filter docs…" value={filter} onChange={(e) => setFilter(e.target.value)} />
        </label>
        {nav.map((g) => (
          <div key={g.title} className={styles.navGroup}>
            <div className={styles.navTitle}>{g.title}</div>
            {g.items.map((i) => (
              <a
                key={i.id}
                href={`#${i.id}`}
                data-nav={i.id}
                className={`${styles.navLink} ${active === i.id ? styles.navActive : ""}`}
                onClick={() => setNavOpen(false)}
              >
                {i.method && <Method m={i.method} />}
                <span>{i.label}</span>
              </a>
            ))}
          </div>
        ))}
      </aside>

      <main className={styles.content}>
        {/* ------------------------------------------------ Introduction */}
        <Section id="introduction" split>
          <div className={styles.prose}>
            <div className={styles.kicker}>PayTrack API reference</div>
            <h1>Collect M-Pesa payments with one API call</h1>
            <p>
              The PayTrack API is a REST API for sending M-Pesa STK push prompts and routing the money straight into your bank account,
              PayBill or Till. You don&apos;t need your own Daraja app: PayTrack sends every push from its platform app, tracks it to a final
              state, and tells your backend with a signed webhook.
            </p>
            <p>
              Requests and responses are JSON, every response uses standard HTTP status codes, and every request is authenticated with a
              bearer token.
            </p>
            <div className={styles.factGrid}>
              <div><span>Base URL</span><code>{BASE}</code></div>
              <div><span>Dashboard</span><a href={DASHBOARD_URL}>{DASHBOARD_URL.replace("https://", "")}</a></div>
              <div><span>Format</span>JSON over HTTPS, UTF-8</div>
              <div><span>Currency</span>KES, whole shillings</div>
              <div><span>Timestamps</span>RFC 3339, UTC</div>
              <div><span>IDs</span>Prefixed, time-sortable: <code>pay_</code>, <code>chn_</code>, <code>evt_</code>…</div>
            </div>
          </div>
          <div className={styles.aside}>
            <Code title="Base URL" lang="sh" code={BASE} />
            <Code
              title="Every response carries a request ID"
              lang="sh"
              code={`HTTP/1.1 201 Created\nContent-Type: application/json\nX-Request-Id: req_01k7x5j2p3q4r5s6t7v8w9x0yz`}
            />
          </div>
        </Section>

        {/* ------------------------------------------------ Quickstart */}
        <Section id="quickstart" split>
          <div className={styles.prose}>
            <h2>Quickstart</h2>
            <p>From zero to a paid order in four steps.</p>
            <ol className={styles.steps}>
              <li>
                <strong>Create an account and an API key.</strong> Sign up in the <Link href="/auth/register">dashboard</Link>, then go to
                {" "}<Link href="/dashboard/api-keys">API keys → Create key</Link>. Copy the <code>ptk_…</code> key into your backend&apos;s secrets
                as <code>PAYTRACK_API_KEY</code>. It&apos;s shown once.
              </li>
              <li>
                <strong>Add a channel.</strong> Tell PayTrack where the money should land: a bank account, a PayBill or a Till. Keep the
                returned <code>chn_…</code> ID.
              </li>
              <li>
                <strong>Set your webhook URL</strong> and copy the <code>whsec_…</code> signing secret into <code>PAYTRACK_SIGNING_SECRET</code>.
              </li>
              <li>
                <strong>Charge the customer.</strong> Create a payment with your order ID as the <code>Idempotency-Key</code>. The customer gets
                a prompt; you get a <code>payment.succeeded</code> webhook when they pay.
              </li>
            </ol>
            <Callout>
              Never call the PayTrack API from a browser or mobile app. API keys can move money into your channels and must stay on your server.
            </Callout>
          </div>
          <div className={styles.aside}>
            <RequestSample lang={lang} setLang={setLang} title="2. Add a channel" ep={ENDPOINT_GROUPS[1].endpoints[1]} />
            <RequestSample lang={lang} setLang={setLang} title="3. Set the webhook URL" ep={ENDPOINT_GROUPS[2].endpoints[1]} />
            <RequestSample lang={lang} setLang={setLang} title="4. Charge the customer" ep={ENDPOINT_GROUPS[0].endpoints[0]} />
          </div>
        </Section>

        {/* ------------------------------------------------ Authentication */}
        <Section id="authentication" split>
          <div className={styles.prose}>
            <h2>Authentication</h2>
            <p>
              Send a bearer token in the <code>Authorization</code> header of every request. There are two kinds:
            </p>
            <table className={styles.table}>
              <thead><tr><th>Token</th><th>Looks like</th><th>Use it for</th></tr></thead>
              <tbody>
                <tr><td>API key</td><td><code>ptk_…</code></td><td>Your backend. Doesn&apos;t expire; revoke it from the dashboard.</td></tr>
                <tr><td>Session</td><td>JWT</td><td>The dashboard. Returned by login/register, valid for 12 hours.</td></tr>
              </tbody>
            </table>
            <p>
              Both work on most endpoints. A few account-level endpoints (creating, listing and revoking API keys, rotating the webhook secret)
              are <strong>dashboard session only</strong> and return <code>403 user_session_required</code> for API keys, so a leaked key
              can&apos;t mint new keys or lock you out of your webhooks.
            </p>
            <p>
              Missing or bad tokens return <code>401</code> with <code>missing_token</code>, <code>invalid_api_key</code> or <code>invalid_token</code>.
            </p>
            <Callout tone="warn">
              Treat API keys like passwords. Use one key per environment or service, name them clearly, and revoke any key you suspect has leaked.
              <code> last_used_at</code> on each key shows whether it&apos;s still in use.
            </Callout>
          </div>
          <div className={styles.aside}>
            <RequestSample lang={lang} setLang={setLang} title="Authenticated request" ep={ENDPOINT_GROUPS[3].endpoints[0]} />
            <Code title="401 Unauthorized" code={JSON.stringify({ error: { code: "invalid_api_key", message: "API key is invalid or revoked", request_id: "req_01k7x5j2p3q4r5s6t7v8w9x0yz" } }, null, 2)} />
          </div>
        </Section>

        {/* ------------------------------------------------ Errors */}
        <Section id="errors" split>
          <div className={styles.prose}>
            <h2>Errors</h2>
            <p>
              PayTrack uses conventional HTTP status codes: <code>2xx</code> succeeded, <code>4xx</code> means the request needs fixing,
              <code> 5xx</code> means something went wrong on our side. Every error has the same shape, with a stable machine-readable
              <code> code</code> to branch on and a human-readable <code>message</code> that may change.
            </p>
            <table className={styles.table}>
              <thead><tr><th>Status</th><th>Meaning</th></tr></thead>
              <tbody>{ERROR_STATUSES.map(([s, d]) => <tr key={s}><td className={styles.nowrap}>{s}</td><td><Md>{d}</Md></td></tr>)}</tbody>
            </table>
            <h3>Error codes</h3>
            <table className={styles.table}>
              <thead><tr><th>Code</th><th>Status</th><th>Description</th></tr></thead>
              <tbody>
                {ERROR_CODES.map(([c, s, d]) => (
                  <tr key={c}><td><code>{c}</code></td><td className={styles.nowrap}>{s}</td><td><Md>{d}</Md></td></tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className={styles.aside}>
            <Code
              title="Error object"
              code={JSON.stringify({ error: { code: "invalid_amount", message: "amount must be a whole number of KES between 1 and 250000", request_id: "req_01k7x5j2p3q4r5s6t7v8w9x0yz" } }, null, 2)}
            />
            <Callout>Include the <code>request_id</code> when contacting support. It&apos;s also in the <code>X-Request-Id</code> response header.</Callout>
          </div>
        </Section>

        {/* ------------------------------------------------ Pagination */}
        <Section id="pagination" split>
          <div className={styles.prose}>
            <h2>Pagination</h2>
            <p>
              List endpoints for payments and deliveries use cursor pagination and return objects newest first. Each page has:
            </p>
            <ul>
              <li><code>data</code>: the objects on this page.</li>
              <li><code>has_more</code>: whether older objects exist.</li>
              <li><code>next_cursor</code>: pass it as <code>cursor</code> to get the next page. Omitted on the last page.</li>
            </ul>
            <p>Use <code>limit</code> (1–100, default 25) to set the page size. Cursors stay valid while new objects are created, so you never skip or repeat objects.</p>
          </div>
          <div className={styles.aside}>
            <Code
              title="Iterate every succeeded payment (Node.js)"
              lang="js"
              code={`let cursor;
do {
  const url = new URL("${BASE}/v1/payments");
  url.searchParams.set("status", "succeeded");
  url.searchParams.set("limit", "100");
  if (cursor) url.searchParams.set("cursor", cursor);

  const page = await fetch(url, {
    headers: { Authorization: \`Bearer \${process.env.PAYTRACK_API_KEY}\` },
  }).then((r) => r.json());

  for (const payment of page.data) reconcile(payment);
  cursor = page.has_more ? page.next_cursor : undefined;
} while (cursor);`}
            />
          </div>
        </Section>

        {/* ------------------------------------------------ Idempotency */}
        <Section id="idempotency" split>
          <div className={styles.prose}>
            <h2>Idempotency</h2>
            <p>
              Networks fail. To retry <code>POST /v1/payments</code> safely, send an <code>Idempotency-Key</code> header (required, up to
              255 characters). PayTrack remembers the key for your account:
            </p>
            <ul>
              <li><strong>Same key, same body:</strong> no new push is sent. You get the original payment with <code>200</code> and <code>Idempotent-Replayed: true</code>.</li>
              <li><strong>Same key, different body:</strong> <code>409 idempotency_key_reused</code>. Nothing is charged.</li>
              <li><strong>New key:</strong> a new payment and a new prompt to the customer.</li>
            </ul>
            <p>
              Use something that identifies the <em>charge attempt</em>, such as your order ID, or <code>order-1042-attempt-2</code> when
              you deliberately want to prompt the customer again after a failure.
            </p>
            <Callout tone="warn">
              Retry on timeouts and <code>5xx</code> with the <strong>same</strong> key. Generating a fresh key on every retry can send the
              customer two prompts and charge them twice.
            </Callout>
          </div>
          <div className={styles.aside}>
            <Code title="Replayed response headers" lang="sh" code={`HTTP/1.1 200 OK\nIdempotent-Replayed: true\nX-Request-Id: req_01k7x5k9…`} />
            <Code title="409 Conflict" code={JSON.stringify({ error: { code: "idempotency_key_reused", message: "Idempotency-Key was already used with a different request body", request_id: "req_01k7x5j2p3q4r5s6t7v8w9x0yz" } }, null, 2)} />
          </div>
        </Section>

        {/* ------------------------------------------------ Phones & amounts */}
        <Section id="phone-amounts" split>
          <div className={styles.prose}>
            <h2>Phones &amp; amounts</h2>
            <h3>Phone numbers</h3>
            <p>
              Send the customer&apos;s Safaricom number in whatever format you have. PayTrack strips spaces and symbols and normalises it to
              <code> 2547XXXXXXXX</code> or <code>2541XXXXXXXX</code>, which is the format you&apos;ll see on the payment.
            </p>
            <table className={styles.table}>
              <thead><tr><th>You send</th><th>Stored as</th></tr></thead>
              <tbody>
                {[["0712345678", "254712345678"], ["712345678", "254712345678"], ["+254 712 345 678", "254712345678"], ["0110123456", "254110123456"]].map(([a, b]) => (
                  <tr key={a}><td><code>{a}</code></td><td><code>{b}</code></td></tr>
                ))}
              </tbody>
            </table>
            <h3>Amounts</h3>
            <p>
              <code>amount</code> is an integer number of Kenyan shillings, from 1 to 250,000 (the M-Pesa per-transaction limit).
              M-Pesa doesn&apos;t support cents, so <code>1500</code> means KES 1,500.
            </p>
            <h3>Text limits</h3>
            <p>
              <code>account_reference</code>: 1–12 characters, letters, digits, space and <code>-_/.</code>.
              {" "}<code>description</code>: up to 13 characters. Both limits come from Safaricom.
            </p>
          </div>
          <div className={styles.aside}>
            <Code title="422 Unprocessable Entity" code={JSON.stringify({ error: { code: "invalid_phone", message: "phone must be a Safaricom number, e.g. 0712345678 or 254712345678", request_id: "req_01k7x5j2p3q4r5s6t7v8w9x0yz" } }, null, 2)} />
          </div>
        </Section>

        {/* ------------------------------------------------ Lifecycle */}
        <Section id="lifecycle">
          <div className={styles.prose}>
            <h2>Payment lifecycle</h2>
            <p>
              A payment moves through these statuses. <code>succeeded</code>, <code>failed</code> and <code>cancelled</code> are final.
              Every move into a final (or expired) state sends you a webhook.
            </p>
          </div>
          <div className={styles.flow}>
            <div className={styles.flowCol}>
              <div className={`${styles.state} ${styles.stWarn}`}>created</div>
              <div className={styles.flowNote}>Saved; push being sent or Safaricom&apos;s answer was ambiguous</div>
            </div>
            <div className={styles.arrow}>→</div>
            <div className={styles.flowCol}>
              <div className={`${styles.state} ${styles.stWarn}`}>pending</div>
              <div className={styles.flowNote}>Prompt on the customer&apos;s phone</div>
            </div>
            <div className={styles.arrow}>→</div>
            <div className={styles.flowCol}>
              <div className={`${styles.state} ${styles.stOk}`}>succeeded</div>
              <div className={`${styles.state} ${styles.stErr}`}>failed</div>
              <div className={`${styles.state} ${styles.stErr}`}>cancelled</div>
              <div className={`${styles.state} ${styles.stMuted}`}>expired</div>
            </div>
          </div>
          <div className={styles.prose}>
            <table className={styles.table}>
              <thead><tr><th>Status</th><th>Meaning</th><th>Final?</th><th>Webhook</th></tr></thead>
              <tbody>
                <tr><td><code>created</code></td><td>Payment saved. Either the push is in flight, or Safaricom&apos;s response was ambiguous (a timeout or 5xx doesn&apos;t prove the prompt wasn&apos;t sent).</td><td>No</td><td>none</td></tr>
                <tr><td><code>pending</code></td><td>Safaricom accepted the push; waiting for the customer to enter their PIN.</td><td>No</td><td>none</td></tr>
                <tr><td><code>succeeded</code></td><td>Money received at your destination. <code>mpesa_receipt_number</code> is set when Safaricom provides it.</td><td>Yes</td><td><code>payment.succeeded</code></td></tr>
                <tr><td><code>failed</code></td><td>Safaricom rejected the push or the payment failed (e.g. insufficient funds, wrong PIN). See <code>result_desc</code> / <code>failure_reason</code>.</td><td>Yes</td><td><code>payment.failed</code></td></tr>
                <tr><td><code>cancelled</code></td><td>The customer dismissed the prompt (result code 1032).</td><td>Yes</td><td><code>payment.cancelled</code></td></tr>
                <tr><td><code>expired</code></td><td>PayTrack couldn&apos;t learn the outcome in time. <strong>Can still become <code>succeeded</code></strong> if proof of payment arrives later.</td><td>Almost</td><td><code>payment.expired</code></td></tr>
              </tbody>
            </table>
            <h3>How PayTrack finds the outcome</h3>
            <ul>
              <li><strong>Callbacks first.</strong> Safaricom calls PayTrack when the customer responds, usually within 10–60 seconds.</li>
              <li><strong>Status queries as backup.</strong> If no callback arrives, PayTrack queries Safaricom at 45s, 1m, 2m, 5m, 10m and 20m, then expires the payment.</li>
              <li><strong>Ambiguous pushes.</strong> A payment still <code>created</code> after 3 minutes with no callback expires.</li>
              <li><strong>Late success wins.</strong> If an expired payment later turns out to have been paid, it moves to <code>succeeded</code> and you get <code>payment.succeeded</code>, so money that arrived is never reported as lost.</li>
            </ul>
            <Callout tone="warn">
              Treat <code>payment.expired</code> as &ldquo;unknown&rdquo;, not &ldquo;unpaid&rdquo;. Don&apos;t ship goods, but don&apos;t re-charge
              automatically either. Release the order if <code>payment.succeeded</code> arrives later.
            </Callout>
          </div>
        </Section>

        {/* ------------------------------------------------ Routing */}
        <Section id="routing" split>
          <div className={styles.prose}>
            <h2>Routing money</h2>
            <p>
              Every channel has a <code>type</code> that decides where the money goes and how <code>account_reference</code> on a payment behaves.
            </p>
            <table className={styles.table}>
              <thead><tr><th>Type</th><th>Money goes to</th><th>Create with</th><th><code>account_reference</code> on payments</th></tr></thead>
              <tbody>
                <tr>
                  <td><code>bank</code></td>
                  <td>Your bank account, via the bank&apos;s PayBill</td>
                  <td><code>bank_code</code> from <a href="#list-banks">List banks</a> + <code>account_number</code></td>
                  <td><strong>Omit it.</strong> Always the channel&apos;s account number; a different value returns <code>account_reference_mismatch</code>.</td>
                </tr>
                <tr>
                  <td><code>paybill</code></td>
                  <td>Any PayBill</td>
                  <td><code>business_number</code> + optional default <code>account_number</code></td>
                  <td>Overrides the default. <strong>Required</strong> if the channel has no default (<code>account_reference_required</code>).</td>
                </tr>
                <tr>
                  <td><code>till</code></td>
                  <td>A Buy Goods Till</td>
                  <td><code>business_number</code></td>
                  <td>Optional; defaults to <code>Till Payment</code>. Buy Goods ignores it.</td>
                </tr>
              </tbody>
            </table>
            <p>
              Each payment keeps a snapshot of its routing (<code>destination_type</code>, <code>business_number</code>, <code>bank_name</code>),
              so deactivating or replacing a channel never rewrites past payments.
            </p>
          </div>
          <div className={styles.aside}>
            <Code
              title="Charging each channel type"
              lang="js"
              code={`// bank: account_reference is always the channel's account
{ "channel_id": "${EX.channelBank.id}", "amount": 1500, "phone": "0712345678" }

// paybill: per-payment account, e.g. the customer's meter number
{ "channel_id": "${EX.channelPaybill.id}", "amount": 2000, "phone": "0712345678",
  "account_reference": "METER-55102" }

// till: nothing extra
{ "channel_id": "${EX.channelTill.id}", "amount": 350, "phone": "0712345678" }`}
            />
          </div>
        </Section>

        {/* ------------------------------------------------ Webhooks */}
        <Section id="webhooks" split>
          <div className={styles.prose}>
            <h2>Webhooks</h2>
            <p>
              When a payment reaches a final or expired state, PayTrack <code>POST</code>s an <a href="#object-event">Event</a> to your webhook URL.
              Webhooks are the reliable way to fulfil orders; polling is a fallback.
            </p>
            <h3>Event types</h3>
            <table className={styles.table}>
              <thead><tr><th>Type</th><th>Sent when</th></tr></thead>
              <tbody>
                <tr><td><code>payment.succeeded</code></td><td>The customer paid. Fulfil the order. Can follow a <code>payment.expired</code>.</td></tr>
                <tr><td><code>payment.failed</code></td><td>The push was rejected or the payment failed.</td></tr>
                <tr><td><code>payment.cancelled</code></td><td>The customer dismissed the prompt.</td></tr>
                <tr><td><code>payment.expired</code></td><td>No outcome could be determined in time.</td></tr>
              </tbody>
            </table>
            <h3>Request headers</h3>
            <table className={styles.table}>
              <thead><tr><th>Header</th><th>Value</th></tr></thead>
              <tbody>
                <tr><td><code>PayTrack-Event-Id</code></td><td>The event ID (<code>evt_…</code>). Same across retries; dedupe on it.</td></tr>
                <tr><td><code>PayTrack-Event-Type</code></td><td>e.g. <code>payment.succeeded</code></td></tr>
                <tr><td><code>PayTrack-Delivery-Id</code></td><td>The delivery ID (<code>whd_…</code>), for matching against the dashboard.</td></tr>
                <tr><td><code>PayTrack-Signature</code></td><td><code>t=&lt;unix seconds&gt;,v1=&lt;hex HMAC-SHA256&gt;</code></td></tr>
                <tr><td><code>User-Agent</code></td><td><code>PayTrack-Webhooks/1</code></td></tr>
                <tr><td><code>Content-Type</code></td><td><code>application/json</code></td></tr>
              </tbody>
            </table>
            <h3>Verifying signatures</h3>
            <p>Anyone can POST to your URL, so verify every request before trusting it:</p>
            <ol className={styles.steps}>
              <li>Split <code>PayTrack-Signature</code> on <code>,</code> and read <code>t</code> and <code>v1</code>.</li>
              <li>Compute <code>HMAC-SHA256(signing_secret, t + &quot;.&quot; + raw_body)</code> as lowercase hex. Use the <strong>raw</strong> bytes, not re-serialised JSON.</li>
              <li>Compare it with <code>v1</code> in constant time.</li>
              <li>Reject if <code>t</code> is more than 5 minutes from now, to block replays.</li>
            </ol>
            <h3>Responding and retries</h3>
            <p>
              Return any <code>2xx</code> within 10 seconds to acknowledge. Anything else (non-2xx, redirects, timeouts, connection errors) is
              retried with backoff for about 16 hours:
            </p>
            <div className={styles.retryRow}>
              <span className={styles.retryFirst}>attempt 1</span>
              {WEBHOOK_RETRY_SCHEDULE.map((d, i) => <span key={i}>+{d}</span>)}
            </div>
            <p>
              After the last retry the delivery is marked <code>failed</code>. You can replay any delivery from the dashboard or with
              {" "}<a href="#redeliver">Redeliver an event</a>, which resets its retry budget. Each attempt uses your <em>current</em> URL and secret.
            </p>
            <h3>Best practices</h3>
            <ul>
              <li><strong>Dedupe on <code>PayTrack-Event-Id</code>.</strong> Delivery is at-least-once, so the same event can arrive more than once.</li>
              <li><strong>Acknowledge fast.</strong> Store the event, return <code>200</code>, and do slow work (emails, ERP sync) in a background job.</li>
              <li><strong>Don&apos;t rely on order.</strong> Use <code>data.status</code> and <code>data.updated_at</code> as the truth, not arrival order.</li>
              <li><strong>Match orders with <code>metadata</code>.</strong> Put your order ID in <code>metadata</code> when creating the payment.</li>
              <li><strong>Use a public HTTPS URL.</strong> Endpoints that resolve to private or loopback IPs are refused. Redirects are treated as failures.</li>
            </ul>
          </div>
          <div className={styles.aside}>
            <Code
              title="POST to your webhook URL"
              lang="sh"
              code={`POST /hooks/paytrack HTTP/1.1\nHost: acme.co.ke\nContent-Type: application/json\nUser-Agent: PayTrack-Webhooks/1\nPayTrack-Event-Id: ${EX.event.id}\nPayTrack-Event-Type: payment.succeeded\nPayTrack-Delivery-Id: ${EX.delivery.id}\nPayTrack-Signature: t=1760520279,v1=5f2b9c1e0a7d4b38e6f19a2c7d0e4b85a3c6f1d9e2b7a40c58d3e1f6a9b2c7d4`}
            />
            <Code title="Event body" code={JSON.stringify(EX.event, null, 2)} />
            <VerifySample lang={lang} />
          </div>
        </Section>

        {/* ------------------------------------------------ Go-live */}
        <Section id="go-live">
          <div className={styles.prose}>
            <h2>Go-live checklist</h2>
            <ul className={styles.checklist}>
              <li>API keys are stored server-side only, one per environment, with clear names.</li>
              <li>Every <code>POST /v1/payments</code> sends a stable <code>Idempotency-Key</code>, and retries reuse it.</li>
              <li>Your webhook endpoint verifies <code>PayTrack-Signature</code> against the raw body and checks the timestamp.</li>
              <li>Events are deduplicated on <code>id</code> and acknowledged within 10 seconds.</li>
              <li>Orders are fulfilled on <code>payment.succeeded</code> only, and <code>payment.expired</code> is handled as &ldquo;unknown&rdquo;.</li>
              <li>You log <code>request_id</code> on API errors and branch on <code>error.code</code>, not <code>message</code>.</li>
              <li>A nightly job lists recent payments to reconcile against your orders, in case webhooks were missed.</li>
              <li>The webhook signing secret is in your secret manager, and you know how to rotate it.</li>
            </ul>
          </div>
        </Section>

        {/* ------------------------------------------------ Objects */}
        {OBJECTS.map((o) => (
          <Section key={o.id} id={o.id} split>
            <div className={styles.prose}>
              <div className={styles.kicker}>Object</div>
              <h2>{o.name}</h2>
              <p><Md>{o.intro}</Md></p>
              <Fields rows={o.fields} />
            </div>
            <div className={styles.aside}>
              <Code title={o.name} code={JSON.stringify(o.example, null, 2)} />
            </div>
          </Section>
        ))}

        {/* ------------------------------------------------ Endpoints */}
        {ENDPOINT_GROUPS.map((g) => (
          <Fragment key={g.id}>
            <div className={styles.groupHeader} id={`group-${g.id}`}>
              <div className={styles.kicker}>Endpoints</div>
              <h2>{g.title}</h2>
              <p>{g.intro}</p>
            </div>
            {g.endpoints.map((ep) => (
              <Endpoint key={ep.id} ep={ep} lang={lang} setLang={setLang} />
            ))}
          </Fragment>
        ))}

        <footer className={styles.footer}>
          <span>© {new Date().getFullYear()} Paytrack from Geninworld Limited</span>
          <span>
            Machine-readable spec: <code>server/api/openapi.yaml</code> (OpenAPI 3.1)
          </span>
        </footer>
      </main>

      {navOpen && <div className={styles.scrim} onClick={() => setNavOpen(false)} />}
    </div>
  );
}

function Endpoint({ ep, lang, setLang }) {
  const [variant, setVariant] = useState(0);
  const v = ep.variants?.[variant];
  const req = v ? { ...ep.request, body: v.body } : ep.request;
  const responses = v ? [{ status: "201 Created", body: v.response }, ...ep.responses.slice(1)] : ep.responses;

  return (
    <Section id={ep.id} split>
      <div className={styles.prose}>
        <h3 className={styles.epTitle}>{ep.title}</h3>
        <div className={styles.epPath}>
          <Method m={ep.method} />
          <code>{ep.path}</code>
          <AuthBadge auth={ep.auth} />
        </div>
        <p><Md>{ep.description}</Md></p>
        <Params title="Path parameters" rows={ep.pathParams} />
        <Params title="Headers" rows={ep.headers} />
        <Params title="Query parameters" rows={ep.query} />
        <Params title="Body parameters" rows={ep.body} />
        <div className={styles.returns}>
          <div className={styles.paramsTitle}>Returns</div>
          <p>
            {ep.responses[0].status.startsWith("204")
              ? <>An empty <code>204 No Content</code> response on success.</>
              : <>A <code>{ep.responses[0].status}</code> response with the JSON shown. Errors use the standard <a href="#errors">error object</a>.</>}
          </p>
        </div>
      </div>
      <div className={styles.aside}>
        {ep.variants && (
          <div className={styles.variantTabs}>
            {ep.variants.map((x, i) => (
              <button key={x.label} className={i === variant ? styles.variantActive : ""} onClick={() => setVariant(i)}>{x.label}</button>
            ))}
          </div>
        )}
        <RequestSample ep={ep} req={req} lang={lang} setLang={setLang} />
        <Responses key={variant} responses={responses} />
      </div>
    </Section>
  );
}
