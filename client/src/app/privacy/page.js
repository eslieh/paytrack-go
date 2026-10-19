import Link from "next/link";
import LegalPage from "@/components/LegalPage.jsx";
import s from "@/components/legal.module.css";
import { LEGAL } from "@/lib/legal";
import { pageMeta } from "@/lib/site";

export const metadata = pageMeta({
  title: "Privacy Policy",
  description: "What data Paytrack collects, why, who it's shared with, how it's protected, and your rights under Kenya's Data Protection Act.",
  path: "/privacy",
});

const DATA = [
  ["Account", "Business name, owner email, password (stored as a bcrypt hash, never in plain text), email verification state", "Running your account, signing you in, contacting you", "You"],
  ["Google sign-in", "Google account ID, email, and the name Google shares", "Signing you in with Google, if you choose it", "Google"],
  ["Sessions & security", "IP address, user agent, session and trusted-device records, sign-in and security events", "Keeping accounts secure, detecting abuse, rate limiting, audit trail", "Your browser"],
  ["API keys", "Key name, prefix, a one-way hash of the key, created and last-used times", "Authenticating your backend", "You"],
  ["Channels", "Destination type, bank code, PayBill or Till number, account numbers", "Routing payments where you told us to", "You"],
  ["Payments", "Customer phone number, amount, account reference, description, your metadata, Safaricom IDs, receipt numbers, result codes", "Sending STK pushes, tracking outcomes, webhooks, your dashboard", "You, your Customers, Safaricom"],
  ["Webhooks", "Webhook URL, signing secret (encrypted at rest), delivery attempts, response codes, errors", "Delivering and replaying events to your backend", "You, your server"],
  ["Email", "Messages we send you (verification, security alerts, product notices) and delivery status", "Account and security communication", "Us"],
  ["Logs", "Request IDs, paths, status codes, timings", "Debugging, incident response, capacity planning", "Your requests"],
];

const SUBPROCESSORS = [
  ["Safaricom PLC (M-Pesa Daraja)", "Sending STK pushes and receiving results", "Payment data needed for the push: phone, amount, destination, reference"],
  ["Contabo", "Hosting, compute and database", "Everything we store, encrypted in transit"],
  ["Resend", "Transactional email delivery", "Your email address and the message content"],
  ["Google", "Optional Google sign-in", "Sign-in exchange only, if you use it"],
];

const sections = [
  {
    id: "who",
    title: "Who we are",
    body: (
      <p>
        {LEGAL.product} is operated by {LEGAL.company}, {LEGAL.country}. This policy explains what personal data we process when you use the
        API, dashboard and docs, and what you can do about it. It&apos;s written against Kenya&apos;s Data Protection Act, 2019. Contact for anything
        privacy-related: <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a>.
      </p>
    ),
  },
  {
    id: "roles",
    title: "Controller or processor? It depends whose data",
    body: (
      <table className={s.table}>
        <thead><tr><th>Data about</th><th>Our role</th><th>What that means</th></tr></thead>
        <tbody>
          <tr><td>You (account owners and users)</td><td>Controller</td><td>We decide how it&apos;s used, as described in this policy.</td></tr>
          <tr><td>Your Customers (people who pay you)</td><td>Processor</td><td>You&apos;re the controller. We process their data only to run payments for you. Customers should contact the merchant they paid first.</td></tr>
        </tbody>
      </table>
    ),
  },
  {
    id: "data",
    title: "What we collect",
    body: (
      <>
        <p>Only what&apos;s needed to make payments work and keep them secure:</p>
        <table className={s.table}>
          <thead><tr><th>Category</th><th>Examples</th><th>Why</th><th>Source</th></tr></thead>
          <tbody>{DATA.map(([c, e, w, src]) => <tr key={c}><td><strong>{c}</strong></td><td>{e}</td><td>{w}</td><td>{src}</td></tr>)}</tbody>
        </table>
        <p>
          Never collected: M-Pesa PINs, card numbers, bank login credentials or your customers&apos; account balances. Those stay between your
          Customers, Safaricom and their banks.
        </p>
      </>
    ),
  },
  {
    id: "bases",
    title: "Why we're allowed to (lawful bases)",
    body: (
      <ul>
        <li><strong>Contract:</strong> running your account and the payments you ask for.</li>
        <li><strong>Legitimate interests:</strong> security, fraud prevention, abuse detection, debugging and improving the Service, balanced against your rights.</li>
        <li><strong>Legal obligation:</strong> keeping records and responding to lawful requests from authorities.</li>
        <li><strong>Consent:</strong> optional things like Google sign-in, which you can stop using at any time.</li>
      </ul>
    ),
  },
  {
    id: "not-doing",
    title: "Things we don't do",
    body: (
      <ul>
        <li>We don&apos;t sell personal data or rent it out. Ever.</li>
        <li>We don&apos;t run ad trackers or third-party analytics scripts in the dashboard or docs.</li>
        <li>We don&apos;t contact your Customers for our own purposes or build profiles of them.</li>
        <li>We don&apos;t use your payment data to compete with you or to market to anyone.</li>
      </ul>
    ),
  },
  {
    id: "sharing",
    title: "Who we share it with",
    body: (
      <>
        <p>A short list of subprocessors, each getting only what its job needs:</p>
        <table className={s.table}>
          <thead><tr><th>Provider</th><th>Purpose</th><th>Data involved</th></tr></thead>
          <tbody>{SUBPROCESSORS.map(([p, u, d]) => <tr key={p}><td><strong>{p}</strong></td><td>{u}</td><td>{d}</td></tr>)}</tbody>
        </table>
        <p>
          Outside that list, we also send data to <strong>your own webhook endpoint</strong> (you choose the URL), and we disclose data where
          the law requires it, such as a valid court order. We push back on requests that are overly broad.
        </p>
      </>
    ),
  },
  {
    id: "transfers",
    title: "International transfers",
    body: (
      <p>
        Some providers above may store or process data outside {LEGAL.country}. Where that happens, we rely on providers with strong
        security and contractual safeguards, as Kenyan data-protection law requires for cross-border transfers.
      </p>
    ),
  },
  {
    id: "security",
    title: "Security, in actual technical terms",
    body: (
      <ul>
        <li>TLS on every connection to the API and dashboard.</li>
        <li>Passwords hashed with bcrypt. API keys stored only as one-way hashes, so we can&apos;t show you a key again, and nobody else can either.</li>
        <li>Webhook signing secrets and other sensitive credentials encrypted at rest with a dedicated key.</li>
        <li>Session refresh token in an <code>httpOnly</code> cookie that page JavaScript can&apos;t read. The access token lives only in memory.</li>
        <li>Rate limiting, sign-in checks for unrecognised devices, and an audit log of security events.</li>
        <li>Least-privilege access to production, with secrets kept outside the codebase.</li>
      </ul>
    ),
  },
  {
    id: "retention",
    title: "How long we keep it",
    body: (
      <>
        <p>
          Account, channel and payment records are kept while your account is active. After closure we keep payment and transaction records
          only as long as we need them for legal, accounting, tax or dispute purposes, then delete or anonymise them.
        </p>
        <p>
          Session, device and security records are kept for a limited period for security and then purged. Revoked API keys keep their
          metadata (never the key itself) so your audit trail stays intact.
        </p>
      </>
    ),
  },
  {
    id: "cookies",
    title: "Cookies and local storage",
    body: (
      <table className={s.table}>
        <thead><tr><th>Item</th><th>Type</th><th>Purpose</th></tr></thead>
        <tbody>
          <tr><td>Session refresh cookie</td><td>Strictly necessary, <code>httpOnly</code></td><td>Keeps you signed in across page loads.</td></tr>
          <tr><td>Trusted-device cookie</td><td>Strictly necessary, <code>httpOnly</code></td><td>Skips the extra sign-in check on devices you marked as trusted.</td></tr>
          <tr><td>Google sign-in state cookie</td><td>Strictly necessary, <code>httpOnly</code>, short-lived</td><td>Protects the Google sign-in round trip against forgery. Expires within minutes.</td></tr>
          <tr><td>Docs language preference</td><td><code>localStorage</code></td><td>Remembers whether you like cURL, Node.js, Python or PHP samples.</td></tr>
        </tbody>
      </table>
    ),
  },
  {
    id: "rights",
    title: "Your rights",
    body: (
      <>
        <p>Under the Data Protection Act, 2019 you can ask to:</p>
        <ul>
          <li>be told what data we hold about you, and get a copy;</li>
          <li>correct inaccurate data (most of it you can edit yourself in the dashboard);</li>
          <li>delete data we no longer have a reason to keep;</li>
          <li>object to or restrict processing based on legitimate interests;</li>
          <li>get your data in a portable, machine-readable format (JSON, naturally).</li>
        </ul>
        <p>
          Email <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a> from your account email and we&apos;ll respond within the time the law
          allows. If you&apos;re not happy with our answer, you can complain to the Office of the Data Protection Commissioner (ODPC) in Kenya.
        </p>
        <div className={s.callout}>
          <strong>Paid a merchant through {LEGAL.product}?</strong> The merchant is the controller of your data. Contact them first. If you
          can&apos;t reach them, email us and we&apos;ll help route the request.
        </div>
      </>
    ),
  },
  {
    id: "children",
    title: "Children",
    body: <p>The Service is for businesses and developers. It isn&apos;t aimed at anyone under 18, and we don&apos;t knowingly create accounts for them.</p>,
  },
  {
    id: "breaches",
    title: "If something goes wrong",
    body: (
      <p>
        If we have a personal-data breach that&apos;s likely to put you at risk, we&apos;ll notify the ODPC and affected account owners without
        undue delay, explaining what happened, what data was involved, and what we&apos;re doing about it.
      </p>
    ),
  },
  {
    id: "changes",
    title: "Changes to this policy",
    body: (
      <p>
        This policy is versioned (see the top). We&apos;ll give you advance notice of material changes by email or in the dashboard. Related
        reading: <Link href="/terms">Terms of Service</Link> and the <Link href="/disclaimer">Non-custodial Disclaimer</Link>.
      </p>
    ),
  },
];

export default function PrivacyPage() {
  return (
    <LegalPage
      current="/privacy"
      kicker="Privacy Policy"
      title="Privacy Policy"
      tldr={[
        "We collect what's needed to route payments and keep accounts secure. That's it.",
        "Passwords are bcrypt-hashed, API keys hashed, secrets encrypted at rest.",
        "Your Customers' data is yours. We process it for you, as your processor.",
        "No selling data, no ad trackers, no third-party analytics.",
        "You have rights under Kenya's Data Protection Act 2019. Email us to use them.",
      ]}
      sections={sections}
    />
  );
}
