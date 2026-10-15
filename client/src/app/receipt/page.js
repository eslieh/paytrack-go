"use client";

import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, Download } from "lucide-react";
import { api } from "@/lib/api";
import { fmtDate, fmtKES } from "@/lib/format";
import { useAuth } from "@/context/AuthContext.jsx";

const box = { border: "1px solid #e9ebf4", borderRadius: 12, padding: 12 };
const muted = { color: "#657090" };

export default function ReceiptPage() {
  return (
    <Suspense fallback={null}>
      <Receipt />
    </Suspense>
  );
}

function Receipt() {
  const id = useSearchParams().get("id");
  const { token, tenant, loading } = useAuth();
  const [payment, setPayment] = useState(null);
  const [channel, setChannel] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (loading || !token || !id) return;
    api.payments.get(id)
      .then((p) => {
        setPayment(p);
        return api.channels.get(p.channel_id).then(setChannel).catch(() => {});
      })
      .catch((e) => setError(e.message));
  }, [id, token, loading]);

  let message = "";
  if (!id) message = "No payment specified.";
  else if (!loading && !token) message = "Sign in to view this receipt.";
  else if (error) message = error;
  else if (payment && payment.status !== "succeeded") message = `This payment is ${payment.status}; receipts are only issued for successful payments.`;

  if (message || !payment) {
    return (
      <div style={{ minHeight: "100svh", display: "grid", placeItems: "center", padding: 24 }}>
        <div style={{ ...box, background: "#fff", padding: 24, textAlign: "center", maxWidth: 420 }}>
          <div style={{ marginBottom: 12 }}>{message || "Loading receipt..."}</div>
          {message && <Link className="btn btn-primary" href={token ? "/dashboard/payments" : `/auth?next=${encodeURIComponent(`/receipt?id=${id || ""}`)}`}>{token ? "Back to payments" : "Sign in"}</Link>}
        </div>
      </div>
    );
  }

  const number = channel ? (channel.type === "till" ? `Till ${channel.till_number}` : `PayBill ${channel.short_code}`) : null;

  return (
    <div style={{ minHeight: "100svh", display: "grid", placeItems: "center", padding: 24 }}>
      <div style={{ width: "100%", maxWidth: 720, background: "#fff", color: "#0f1222", border: "1px solid #e9ebf4", borderRadius: 16, boxShadow: "0 12px 30px rgba(12,18,37,0.06)" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: 16, borderBottom: "1px solid #eef1f7" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <CheckCircle2 color="#16a34a" />
            <strong>Payment received</strong>
          </div>
          <button className="btn no-print" onClick={() => window.print()} style={{ border: "1px solid #e9ebf4" }}>
            <Download size={18} />
            Print / Save
          </button>
        </div>

        <div style={{ display: "grid", gap: 12, padding: 16 }}>
          <div className="receipt-grid-two" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div style={box}>
              <div style={{ ...muted, fontSize: 12 }}>Received by</div>
              <div style={{ fontWeight: 700 }}>{tenant?.name || "—"}</div>
              {number && <div style={muted}>{number}</div>}
            </div>
            <div style={box}>
              <div style={{ ...muted, fontSize: 12 }}>M-Pesa receipt</div>
              <div style={{ fontWeight: 700 }}>{payment.mpesa_receipt_number || "—"}</div>
              <div style={muted}>Paid {fmtDate(payment.completed_at)}</div>
            </div>
          </div>

          <div style={{ border: "1px solid #e9ebf4", borderRadius: 12, overflow: "hidden" }}>
            <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", padding: 12, background: "#f9faff", ...muted, fontSize: 12 }}>
              <div>Description</div><div style={{ textAlign: "right" }}>Amount</div>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", padding: 12, borderTop: "1px solid #eef1f7" }}>
              <div>
                {payment.description}
                <div style={{ ...muted, fontSize: 12 }}>Account {payment.account_reference}</div>
              </div>
              <div style={{ textAlign: "right" }}>{fmtKES(payment.amount)}</div>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", padding: 12, borderTop: "1px solid #eef1f7", background: "#fbfcff" }}>
              <div style={{ fontWeight: 700 }}>Total paid</div>
              <div style={{ fontWeight: 800, textAlign: "right" }}>{fmtKES(payment.amount)}</div>
            </div>
          </div>

          <div className="receipt-grid-two" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div style={box}>
              <div style={{ ...muted, fontSize: 12 }}>Paid via</div>
              <div>Safaricom M-Pesa (STK Push)</div>
              <div style={muted}>Phone: +{payment.phone}</div>
            </div>
            <div style={box}>
              <div style={{ ...muted, fontSize: 12 }}>Reference</div>
              <div style={{ wordBreak: "break-all" }}>{payment.id}</div>
              <div style={muted}>Requested {fmtDate(payment.created_at)}</div>
            </div>
          </div>

          <div style={{ textAlign: "center", ...muted, fontSize: 12 }}>
            This receipt was generated by Paytrack.
          </div>
        </div>
      </div>

      <style jsx global>{`
        @media print {
          body { background: #fff !important; }
          .no-print { display: none !important; }
        }
        @media (max-width: 680px) {
          .receipt-grid-two { grid-template-columns: 1fr !important; }
        }
      `}</style>
    </div>
  );
}
