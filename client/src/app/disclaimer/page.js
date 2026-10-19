import Link from "next/link";
import LegalPage from "@/components/LegalPage.jsx";
import s from "@/components/legal.module.css";
import { LEGAL } from "@/lib/legal";
import { pageMeta } from "@/lib/site";

export const metadata = pageMeta({
  title: "Non-custodial Disclaimer",
  description: "Paytrack never holds, pools or settles your money. Funds move from your customer's M-Pesa straight to your bank account, PayBill or Till.",
  path: "/disclaimer",
});

const FLOW = `customer's M-Pesa wallet
        │  (customer enters PIN on the STK prompt)
        ▼
   Safaricom M-Pesa  ───────────────►  your bank account / PayBill / Till
        ▲                                      (money lands here, directly)
        │  STK push request + status queries
        │  (instructions and metadata only, no money)
     Paytrack  ──── signed webhook ────►  your backend`;

const sections = [
  {
    id: "short-version",
    title: "The short version",
    body: (
      <>
        <p>
          {LEGAL.product} is software. It sends instructions to Safaricom and reports back what happened. It is a router for
          payment <em>requests</em>, not a place where money sits.
        </p>
        <div className={s.callout}>
          <strong>We never hold your money.</strong> No balances, no float, no pooled account, no settlement batch, no payout queue.
          At no point does a shilling paid by your customer pass through an account owned or controlled by {LEGAL.company}.
        </div>
      </>
    ),
  },
  {
    id: "how-money-moves",
    title: "How money actually moves",
    body: (
      <>
        <p>Here is the data path versus the money path:</p>
        <div className={s.flow}>{FLOW}</div>
        <ol>
          <li>Your backend calls <code>POST /v1/payments</code> with a channel, amount and phone number.</li>
          <li>{LEGAL.product} asks Safaricom to show your customer an STK prompt, with your channel as the destination (the <code>PartyB</code> and account reference).</li>
          <li>The customer approves on their phone. Safaricom moves the funds from their M-Pesa wallet to your destination.</li>
          <li>Safaricom tells {LEGAL.product} the result. {LEGAL.product} records it and sends your backend a signed webhook.</li>
        </ol>
        <p>
          The only things {LEGAL.product} ever touches are request parameters, status codes, receipts and metadata. The value transfer
          happens entirely inside Safaricom&apos;s and your bank&apos;s systems.
        </p>
      </>
    ),
  },
  {
    id: "what-we-are-not",
    title: "What we are not",
    body: (
      <>
        <p>Because we never take possession of funds, {LEGAL.product} is not, and does not act as:</p>
        <ul>
          <li>a bank, deposit-taker or savings product;</li>
          <li>an e-money issuer, wallet or stored-value account;</li>
          <li>an escrow agent, trustee or holder of funds on anyone&apos;s behalf;</li>
          <li>a money remitter or a settlement or payout service;</li>
          <li>a lender or provider of credit, advances or chargeback guarantees.</li>
        </ul>
        <p>
          If any page, email, API field or marketing copy ever seems to suggest otherwise (say, a balance figure in the dashboard), read it
          as a <strong>reporting view computed from payment records</strong>, not as money held by us.
        </p>
      </>
    ),
  },
  {
    id: "what-that-means-for-you",
    title: "What that means for you",
    body: (
      <>
        <ul>
          <li>
            <strong>Refunds and reversals</strong> are between you, your customer, Safaricom and your bank. We can&apos;t pull money back,
            because we never had it. Use the M-Pesa reversal process or refund the customer yourself.
          </li>
          <li>
            <strong>Wrong destination details</strong> (a typo in the account number, the wrong PayBill) send money where you pointed it.
            Double-check your <Link href="/docs#routing">channel configuration</Link>. We have no way to redirect funds.
          </li>
          <li>
            <strong>Settlement timing</strong> depends on Safaricom and your bank, not on us. If funds look delayed, your bank or Safaricom
            statement is the source of truth.
          </li>
          <li>
            <strong>Disputes and chargebacks</strong> belong to you as the merchant. Keep your own records; the
            {" "}<code>mpesa_receipt_number</code> on each payment helps.
          </li>
        </ul>
      </>
    ),
  },
  {
    id: "status-data",
    title: "Status data is information, not a guarantee",
    body: (
      <>
        <p>
          Payment statuses, webhooks and dashboard figures reflect what Safaricom has told us. We work hard to keep them correct
          (callbacks, status queries, idempotency, retries), but distributed systems are distributed systems:
        </p>
        <ul>
          <li>a payment can be <code>expired</code> because the outcome was unknown, and later become <code>succeeded</code>;</li>
          <li>webhooks are delivered at least once, so duplicates are possible and order isn&apos;t guaranteed;</li>
          <li>Safaricom outages can delay or prevent results.</li>
        </ul>
        <p>
          For high-value orders, confirm against your M-Pesa or bank statement before releasing goods. Run a periodic reconciliation
          job; the <Link href="/docs#list-payments">List payments</Link> endpoint exists for exactly this.
        </p>
      </>
    ),
  },
  {
    id: "no-advice",
    title: "No financial, legal or tax advice",
    body: (
      <p>
        Docs, examples and support replies explain how the software works. They aren&apos;t financial, legal, regulatory or tax advice.
        Whether your business needs a licence, how to account for revenue, and which consumer-protection rules apply to you are questions
        for your own advisers.
      </p>
    ),
  },
  {
    id: "third-parties",
    title: "Third parties",
    body: (
      <p>
        M-Pesa is a Safaricom service, and banks, PayBills and Tills are run by their respective operators. Their terms, tariffs, limits and
        outages apply to you independently of ours. {LEGAL.product} isn&apos;t affiliated with or endorsed by Safaricom or any bank,
        and naming them here is purely descriptive.
      </p>
    ),
  },
  {
    id: "related",
    title: "Related documents",
    body: (
      <p>
        This disclaimer forms part of the <Link href="/terms">Terms of Service</Link>. How we handle data is in the
        {" "}<Link href="/privacy">Privacy Policy</Link>. Questions: <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a>.
      </p>
    ),
  },
];

export default function DisclaimerPage() {
  return (
    <LegalPage
      current="/disclaimer"
      kicker="Disclaimer"
      title="We never hold your money"
      tldr={[
        "Paytrack sends STK push instructions and reports results. That's the whole job.",
        "Money goes customer M-Pesa -> Safaricom -> your bank/PayBill/Till. Never through us.",
        "No balances, no float, no payouts, no escrow. We're not a bank, wallet or remitter.",
        "Refunds, reversals and disputes are handled between you, Safaricom and your bank.",
        "Statuses are best-effort information. Reconcile against your statements.",
      ]}
      sections={sections}
    />
  );
}
