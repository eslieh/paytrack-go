import Link from "next/link";
import LegalPage from "@/components/LegalPage.jsx";
import s from "@/components/legal.module.css";
import { LEGAL } from "@/lib/legal";

export const metadata = {
  title: "Terms of Service",
  description: "The rules for using the Paytrack API and dashboard.",
};

const sections = [
  {
    id: "agreement",
    title: "The agreement",
    body: (
      <>
        <p>
          These Terms of Service (the &quot;Terms&quot;) are a contract between you and {LEGAL.company} (&quot;we&quot;, &quot;us&quot;),
          the company behind {LEGAL.product}. They cover the API, the dashboard at <code>{LEGAL.site.replace("https://", "")}</code>, the
          docs, and anything else we ship under the {LEGAL.product} name (together, the &quot;Service&quot;).
        </p>
        <p>
          By creating an account, ticking the box at sign-up, or sending an authenticated request, you accept these Terms, the
          {" "}<Link href="/privacy">Privacy Policy</Link> and the <Link href="/disclaimer">Non-custodial Disclaimer</Link>. If you&apos;re
          accepting for a company, you confirm you&apos;re authorised to bind it, and &quot;you&quot; means that company.
        </p>
      </>
    ),
  },
  {
    id: "definitions",
    title: "Definitions",
    body: (
      <table className={s.table}>
        <thead><tr><th>Term</th><th>Means</th></tr></thead>
        <tbody>
          <tr><td><strong>Account</strong></td><td>Your {LEGAL.product} tenant, including its users, API keys, channels and settings.</td></tr>
          <tr><td><strong>Customer</strong></td><td>The person you send an STK push to, i.e. whoever is paying you.</td></tr>
          <tr><td><strong>Channel</strong></td><td>A destination you configure: a bank account, PayBill or Till.</td></tr>
          <tr><td><strong>Payment</strong></td><td>One STK push request and everything we record about its outcome.</td></tr>
          <tr><td><strong>Your Data</strong></td><td>Data you or your Customers submit through the Service, such as phone numbers, amounts, references and metadata.</td></tr>
        </tbody>
      </table>
    ),
  },
  {
    id: "the-service",
    title: "What the Service does",
    body: (
      <>
        <p>
          {LEGAL.product} lets your software ask Safaricom M-Pesa to prompt a Customer for payment, routes that payment to your Channel,
          tracks it to a final state, and notifies your backend with signed webhooks. The behaviour is documented in the
          {" "}<Link href="/docs">API docs</Link>. Treat them as part of these Terms for technical matters.
        </p>
        <div className={s.callout}>
          {LEGAL.product} is <strong>non-custodial</strong>. We never receive, hold or settle funds, and money moves directly from the
          Customer to your Channel. Details are in the <Link href="/disclaimer">Non-custodial Disclaimer</Link>.
        </div>
      </>
    ),
  },
  {
    id: "accounts",
    title: "Your account",
    body: (
      <ul>
        <li>Give accurate registration details and keep them current, especially your email, which is how we reach you about security and incidents.</li>
        <li>Verify your email before using transacting features. Some endpoints refuse unverified accounts.</li>
        <li>You&apos;re responsible for everything done through your account and its API keys, by you or anyone you let in.</li>
        <li>Use a strong, unique password. Turn on the extra sign-in checks we offer.</li>
        <li>Tell us promptly at <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a> if you suspect unauthorised access.</li>
      </ul>
    ),
  },
  {
    id: "api-keys",
    title: "API keys and integration hygiene",
    body: (
      <>
        <p>API keys (<code>ptk_…</code>) are bearer credentials. Whoever holds one can act as your account. So:</p>
        <ul>
          <li>keep keys server-side, and never ship them in web pages, mobile apps or public repos;</li>
          <li>revoke any key you think has leaked, then create a new one;</li>
          <li>verify webhook signatures and dedupe on event ID, as described in the <Link href="/docs#webhooks">webhook docs</Link>;</li>
          <li>send a stable <code>Idempotency-Key</code> on every payment so retries don&apos;t double-charge.</li>
        </ul>
        <p>Losses caused by a leaked key, unverified webhooks or missing idempotency on your side are your responsibility.</p>
      </>
    ),
  },
  {
    id: "acceptable-use",
    title: "Acceptable use",
    body: (
      <>
        <p>Don&apos;t use the Service to:</p>
        <ul>
          <li>break any law, including Kenyan anti-money-laundering, counter-terrorism-financing, consumer-protection and data-protection law;</li>
          <li>sell illegal goods or services, or run fraud, scams, pyramid schemes or unlicensed gambling or lending;</li>
          <li>send STK prompts to people who haven&apos;t asked to pay you, or spam prompts to pressure anyone into paying;</li>
          <li>impersonate another business, or misdescribe what a Customer is paying for;</li>
          <li>breach Safaricom&apos;s M-Pesa terms or your bank&apos;s terms;</li>
          <li>probe, scan, load-test or attack the Service, get around rate limits, or access accounts that aren&apos;t yours;</li>
          <li>resell or white-label the Service without our written agreement.</li>
        </ul>
        <p>
          Responsible security research is welcome. Email us first, don&apos;t touch other people&apos;s data, and give us reasonable time to fix
          issues before disclosing.
        </p>
      </>
    ),
  },
  {
    id: "merchant-duties",
    title: "You are the merchant",
    body: (
      <ul>
        <li>The sale is between you and your Customer. You handle pricing, delivery, receipts, refunds, complaints and disputes.</li>
        <li>You&apos;re responsible for any licences, tax registrations and invoicing your business needs.</li>
        <li>You check that your Channel details are correct. Money goes exactly where you configure it, and we can&apos;t redirect or recover it.</li>
        <li>You tell your Customers how you process their data, including that you use a payment-routing provider.</li>
      </ul>
    ),
  },
  {
    id: "fees",
    title: "Fees",
    body: (
      <p>
        Safaricom M-Pesa and bank charges apply according to their own tariffs, and those are between you, your Customers and those
        providers. If we introduce {LEGAL.product} fees, we&apos;ll publish them and give you at least 30 days&apos; notice before they apply
        to your account.
      </p>
    ),
  },
  {
    id: "availability",
    title: "Availability and changes",
    body: (
      <>
        <p>
          We aim for high availability and design for failure (retries, reconciliation, idempotency), but we don&apos;t promise the Service will
          be uninterrupted or error-free. It depends on Safaricom, hosting providers and the public internet, none of which we control.
        </p>
        <p>
          We may add, change or remove features. For breaking API changes to <code>/v1</code> we&apos;ll give reasonable advance notice, except
          where a change is needed urgently for security or legal reasons.
        </p>
      </>
    ),
  },
  {
    id: "data",
    title: "Data protection",
    body: (
      <p>
        We handle personal data as described in the <Link href="/privacy">Privacy Policy</Link>. For your Customers&apos; data, you&apos;re the
        data controller and we process it on your instructions, to provide the Service. You confirm you have a lawful basis to share that
        data with us. You own Your Data. You give us the licence we need to host, process and transmit it to run the Service.
      </p>
    ),
  },
  {
    id: "ip",
    title: "Intellectual property",
    body: (
      <p>
        The Service, its code, design, docs and brand belong to {LEGAL.company}. We grant you a limited, non-exclusive, non-transferable
        right to use it while your account is in good standing. You can copy code samples from the docs into your own integration freely.
        If you send us feedback, we may use it without owing you anything.
      </p>
    ),
  },
  {
    id: "suspension",
    title: "Suspension and termination",
    body: (
      <>
        <p>
          You can stop using the Service and close your account at any time. We may suspend or close an account, or block specific
          Channels or keys, if we reasonably believe it&apos;s needed to stop fraud, a breach of these Terms, a security risk, or harm to
          Customers, Safaricom or us. Where we safely and lawfully can, we&apos;ll tell you why.
        </p>
        <p>
          Because we hold no funds, closing an account never traps your money. Payments already in flight still finish at Safaricom, and
          we&apos;ll keep records as required by law (see the Privacy Policy).
        </p>
      </>
    ),
  },
  {
    id: "warranty",
    title: "Disclaimer of warranties",
    body: (
      <p>
        Beyond the commitments written in these Terms, the Service is provided <strong>&quot;as is&quot; and &quot;as available&quot;</strong>.
        To the fullest extent the law allows, we disclaim implied warranties of merchantability, fitness for a particular purpose and
        non-infringement. Payment statuses are information, not a guarantee of funds; see the <Link href="/disclaimer">Disclaimer</Link>.
      </p>
    ),
  },
  {
    id: "liability",
    title: "Limitation of liability",
    body: (
      <>
        <p>To the fullest extent the law allows:</p>
        <ul>
          <li>we aren&apos;t liable for indirect, incidental or consequential losses, or for lost profits, revenue, goodwill or data;</li>
          <li>we aren&apos;t liable for acts or failures of Safaricom, banks, hosting providers or other third parties;</li>
          <li>our total liability for all claims in any 12-month period is capped at the greater of the fees you paid us in that period and KES 10,000.</li>
        </ul>
        <p>Nothing here limits liability that the law says can&apos;t be limited, such as liability for fraud.</p>
      </>
    ),
  },
  {
    id: "indemnity",
    title: "Indemnity",
    body: (
      <p>
        You&apos;ll defend and indemnify {LEGAL.company} against third-party claims arising from your products or services, your dealings with
        your Customers, your breach of these Terms, or your breach of law. That includes claims from Customers about goods, refunds or
        prompts they didn&apos;t ask for.
      </p>
    ),
  },
  {
    id: "changes",
    title: "Changes to these Terms",
    body: (
      <p>
        We version these Terms (see the version at the top). For material changes we&apos;ll notify you by email or in the dashboard at least 14
        days before they take effect. If you keep using the Service after that, you accept the new version. If you don&apos;t agree, close your
        account before the change takes effect.
      </p>
    ),
  },
  {
    id: "law",
    title: "Governing law and disputes",
    body: (
      <p>
        These Terms are governed by the laws of {LEGAL.country}. Before going formal, email us and we&apos;ll try to resolve things in good
        faith within 30 days. If that fails, the courts of {LEGAL.country} have exclusive jurisdiction.
      </p>
    ),
  },
  {
    id: "misc",
    title: "The boilerplate",
    body: (
      <ul>
        <li>If one clause turns out to be unenforceable, the rest still applies.</li>
        <li>Not enforcing a clause right away doesn&apos;t mean we&apos;ve waived it.</li>
        <li>You can&apos;t transfer these Terms without our consent. We may transfer them as part of a merger or sale of the business.</li>
        <li>These Terms, the Privacy Policy and the Disclaimer are the whole agreement about the Service.</li>
        <li>Contact: <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a>.</li>
      </ul>
    ),
  },
];

export default function TermsPage() {
  return (
    <LegalPage
      current="/terms"
      kicker="Terms of Service"
      title="Terms of Service"
      tldr={[
        "You build on our API. Keep your keys server-side and verify your webhooks.",
        "We route M-Pesa payment requests. We never hold your money (see the Disclaimer).",
        "You're the merchant: refunds, receipts, disputes and licences are yours.",
        "No fraud, no unsolicited STK prompts, no attacking the API.",
        "Best-effort service, liability capped, Kenyan law applies.",
      ]}
      sections={sections}
    />
  );
}
