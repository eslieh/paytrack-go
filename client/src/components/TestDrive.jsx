"use client";

import { useEffect, useRef, useState } from "react";
import { CheckCircle2, Play, RotateCcw, XCircle } from "lucide-react";
import styles from "@/app/page.module.css";

// TestDrive is a scripted simulation of one payment's lifecycle for the landing
// page. Nothing is sent anywhere: it replays the same states and events the real
// API produces, so developers can see the flow before they sign up.
const SCRIPTS = {
  pays: [
    { at: 0, status: "created", log: ["POST /v1/payments", "201 Created · Idempotency-Key: order-1042"] },
    { at: 700, status: "pending", log: ["STK prompt shown on 2547•••678", "waiting for the customer's PIN…"] },
    { at: 2600, status: "succeeded", receipt: "TJK3Q8XD2F", log: ["M-Pesa confirmed · receipt TJK3Q8XD2F"] },
    { at: 3200, webhook: true, log: ["POST your.app/hooks/paytrack · payment.succeeded", "signature verified · 200 OK"] },
  ],
  cancels: [
    { at: 0, status: "created", log: ["POST /v1/payments", "201 Created · Idempotency-Key: order-1043"] },
    { at: 700, status: "pending", log: ["STK prompt shown on 2547•••678", "waiting for the customer's PIN…"] },
    { at: 2400, status: "cancelled", log: ["customer dismissed the prompt · result 1032"] },
    { at: 3000, webhook: true, log: ["POST your.app/hooks/paytrack · payment.cancelled", "signature verified · 200 OK"] },
  ],
};

const TONE = { created: "warn", pending: "warn", succeeded: "ok", cancelled: "err" };

export default function TestDrive() {
  const [state, setState] = useState({ status: null, receipt: null, webhook: false, log: [] });
  const [running, setRunning] = useState(false);
  const timers = useRef([]);
  const logRef = useRef(null);

  useEffect(() => () => timers.current.forEach(clearTimeout), []);
  useEffect(() => { logRef.current?.scrollTo({ top: logRef.current.scrollHeight, behavior: "smooth" }); }, [state.log.length]);

  function run(kind) {
    timers.current.forEach(clearTimeout);
    timers.current = [];
    setRunning(true);
    setState({ status: null, receipt: null, webhook: false, log: [] });
    const script = SCRIPTS[kind];
    script.forEach((step, i) => {
      timers.current.push(setTimeout(() => {
        setState((s) => ({
          status: step.status || s.status,
          receipt: step.receipt || s.receipt,
          webhook: s.webhook || Boolean(step.webhook),
          log: [...s.log, ...step.log.map((text) => ({ t: (step.at / 1000).toFixed(1), text, key: `${i}-${text}` }))],
        }));
        if (i === script.length - 1) setRunning(false);
      }, step.at));
    });
  }

  const idle = !state.status;

  return (
    <div className={styles.drive}>
      <div className={styles.driveHead}>
        <span className={styles.driveDots}><i /><i /><i /></span>
        <span>live test drive</span>
        <span className={styles.simTag}>simulated · no money moves</span>
      </div>

      <div className={styles.driveBody}>
        <div className={styles.driveRow}><span>Amount</span><strong className={styles.mono}>KES 250.00</strong></div>
        <div className={styles.driveRow}>
          <span>Status</span>
          {idle ? <span className={styles.muted}>not started</span> : (
            <span key={state.status} className={`${styles.statusPill} ${styles[TONE[state.status]]}`}>{state.status}</span>
          )}
        </div>
        <div className={styles.driveRow}><span>Destination</span><strong>Equity Bank · 0123456789</strong></div>
        <div className={styles.driveRow}>
          <span>Receipt</span>
          <strong className={styles.mono}>{state.receipt || "—"}</strong>
        </div>
        <div className={styles.driveRow}>
          <span>Webhook</span>
          {state.webhook ? <span className={`${styles.statusPill} ${styles.ok}`}>delivered</span> : <span className={styles.muted}>—</span>}
        </div>

        <div className={styles.log} ref={logRef} aria-live="polite">
          {state.log.length === 0 && <div className={styles.logEmpty}>$ press a button to send a test charge_</div>}
          {state.log.map((l) => (
            <div key={l.key} className={styles.logLine}><span className={styles.logT}>+{l.t}s</span>{l.text}</div>
          ))}
        </div>

        <div className={styles.driveActions}>
          <button className={`${styles.driveBtn} ${styles.drivePrimary}`} onClick={() => run("pays")} disabled={running}>
            {idle ? <Play size={15} /> : <RotateCcw size={15} />} Customer pays
          </button>
          <button className={styles.driveBtn} onClick={() => run("cancels")} disabled={running}>
            <XCircle size={15} /> Customer cancels
          </button>
        </div>
        {state.status === "succeeded" && state.webhook && (
          <div className={styles.driveDone}><CheckCircle2 size={15} /> Your backend just got a signed <code>payment.succeeded</code>. Ship the order.</div>
        )}
      </div>
    </div>
  );
}
