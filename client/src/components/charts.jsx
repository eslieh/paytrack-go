"use client";

import { useMemo, useRef, useState } from "react";
import styles from "../app/dashboard/styles.module.css";

// Chart palette (light surface only; the dashboard has no dark theme).
// Categorical slots 1-2 of the reference palette, validated as a pair:
// adjacent CVD ΔE 24.7, normal-vision ΔE 33.6, both >= 3:1 on the surface.
export const SERIES = ["#2a78d6", "#eb6834"];
const GRID = "#eef0f6";
const INK_MUTED = "#8a91ad";

// compact renders 1284 as 1.3K and 4200000 as 4.2M.
export function compact(n) {
  const abs = Math.abs(n);
  if (abs >= 1e9) return `${+(n / 1e9).toFixed(1)}B`;
  if (abs >= 1e6) return `${+(n / 1e6).toFixed(1)}M`;
  if (abs >= 1e3) return `${+(n / 1e3).toFixed(1)}K`;
  return String(Math.round(n));
}

// niceTicks returns 0..max in 3-5 clean steps.
function niceTicks(max) {
  if (max <= 0) return [0, 1];
  const raw = max / 4;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw);
  const top = Math.ceil(max / step) * step;
  const out = [];
  for (let v = 0; v <= top + step / 2; v += step) out.push(v);
  return out;
}

// eachDay fills every date from since (YYYY-MM-DD) for days days, so empty
// days show as gaps at zero instead of being skipped.
export function eachDay(sinceISO, days, rows, pick) {
  const byDate = Object.fromEntries(rows.map((r) => [r.date, r]));
  const start = new Date(`${sinceISO}T00:00:00Z`);
  return Array.from({ length: days }, (_, i) => {
    const d = new Date(start.getTime() + i * 86400000).toISOString().slice(0, 10);
    return { label: d, values: pick(byDate[d]) };
  });
}

const fmtDay = (iso) => new Date(`${iso}T00:00:00Z`).toLocaleDateString(undefined, { day: "numeric", month: "short", timeZone: "UTC" });

// ColumnChart draws (stacked) daily columns with a per-column tooltip and a
// table view. data: [{label: "YYYY-MM-DD", values: [n, ...]}];
// series: [{name}] in stacking order (bottom first).
export function ColumnChart({ data, series, format = compact, height = 200, title }) {
  const [hover, setHover] = useState(null);
  const [table, setTable] = useState(false);
  const wrap = useRef(null);
  const W = 720, H = height, left = 44, right = 8, top = 10, bottom = 24;
  const plotW = W - left - right, plotH = H - top - bottom;

  const totals = data.map((d) => d.values.reduce((a, b) => a + b, 0));
  const ticks = useMemo(() => niceTicks(Math.max(0, ...totals)), [totals.join(",")]); // eslint-disable-line react-hooks/exhaustive-deps
  const yMax = ticks[ticks.length - 1] || 1;
  const slot = plotW / Math.max(1, data.length);
  const barW = Math.max(2, Math.min(24, slot - 2)); // capped at 24px; the leftover is air
  const y = (v) => top + plotH - (v / yMax) * plotH;
  const labelEvery = Math.ceil(data.length / 6);

  function show(i, e) {
    const box = wrap.current.getBoundingClientRect();
    const x = ((left + slot * i + slot / 2) / W) * box.width;
    setHover({ i, x: Math.min(Math.max(x, 80), box.width - 80) });
  }

  return (
    <div>
      <div className={styles.rowBetween} style={{ gap: 12, flexWrap: "wrap", marginBottom: 6 }}>
        {series.length > 1 ? (
          <div style={{ display: "flex", gap: 14, fontSize: 12, color: "#52576e" }} aria-label="Legend">
            {series.map((s, k) => (
              <span key={s.name} style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                <span style={{ width: 10, height: 10, borderRadius: 2, background: SERIES[k] }} />{s.name}
              </span>
            ))}
          </div>
        ) : <span />}
        <button className="btn btn-ghost" style={{ padding: 0, fontSize: 12 }} onClick={() => setTable((v) => !v)}>
          {table ? "Show chart" : "Show table"}
        </button>
      </div>

      {table ? (
        <div style={{ maxHeight: H + 40, overflowY: "auto", border: "1px solid #e9ebf4", borderRadius: 10 }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13, fontVariantNumeric: "tabular-nums" }}>
            <thead><tr style={{ background: "#f9faff", color: "#657090", textAlign: "left" }}>
              <th style={{ padding: "6px 10px", fontWeight: 600 }}>Date</th>
              {series.map((s) => <th key={s.name} style={{ padding: "6px 10px", fontWeight: 600, textAlign: "right" }}>{s.name}</th>)}
            </tr></thead>
            <tbody>{data.map((d) => (
              <tr key={d.label} style={{ borderTop: "1px solid #f1f3fb" }}>
                <td style={{ padding: "6px 10px" }}>{fmtDay(d.label)}</td>
                {d.values.map((v, k) => <td key={k} style={{ padding: "6px 10px", textAlign: "right" }}>{format(v)}</td>)}
              </tr>
            ))}</tbody>
          </table>
        </div>
      ) : (
        <div ref={wrap} style={{ position: "relative" }} onMouseLeave={() => setHover(null)}>
          <svg viewBox={`0 0 ${W} ${H}`} width="100%" role="img" aria-label={title} style={{ display: "block", overflow: "visible" }}>
            {ticks.map((t) => (
              <g key={t}>
                <line x1={left} x2={W - right} y1={y(t)} y2={y(t)} stroke={GRID} strokeWidth="1" />
                <text x={left - 8} y={y(t)} dy="0.32em" textAnchor="end" fontSize="11" fill={INK_MUTED} style={{ fontVariantNumeric: "tabular-nums" }}>{format(t)}</text>
              </g>
            ))}
            {data.map((d, i) => {
              const cx = left + slot * i + slot / 2;
              let base = 0;
              const last = d.values.reduce((acc, v, k) => (v > 0 ? k : acc), -1);
              return (
                <g key={d.label} opacity={hover && hover.i !== i ? 0.55 : 1}>
                  {d.values.map((v, k) => {
                    if (v <= 0) return null;
                    const y0 = y(base), y1 = y(base + v);
                    base += v;
                    // 2px surface gap between stacked segments; 4px rounded top on the last one only.
                    const gap = k > 0 ? 2 : 0;
                    const h = Math.max(1, y0 - y1 - gap);
                    const r = k === last ? Math.min(4, h, barW / 2) : 0;
                    const x0 = cx - barW / 2, yt = y1;
                    const path = r
                      ? `M${x0},${yt + h} V${yt + r} Q${x0},${yt} ${x0 + r},${yt} H${x0 + barW - r} Q${x0 + barW},${yt} ${x0 + barW},${yt + r} V${yt + h} Z`
                      : `M${x0},${yt + h} V${yt} H${x0 + barW} V${yt + h} Z`;
                    return <path key={k} d={path} fill={SERIES[k]} />;
                  })}
                  {i % labelEvery === 0 && (
                    <text x={cx} y={H - 6} textAnchor="middle" fontSize="11" fill={INK_MUTED}>{fmtDay(d.label)}</text>
                  )}
                  {/* The whole column slot is the hit target, bigger than the bar. */}
                  <rect x={left + slot * i} y={top} width={slot} height={plotH} fill="transparent" tabIndex={0}
                    aria-label={`${fmtDay(d.label)}: ${series.map((s, k) => `${s.name} ${format(d.values[k])}`).join(", ")}`}
                    onMouseMove={(e) => show(i, e)} onFocus={(e) => show(i, e)} onBlur={() => setHover(null)} style={{ outline: "none" }} />
                </g>
              );
            })}
            <line x1={left} x2={W - right} y1={y(0)} y2={y(0)} stroke="#d9dce8" strokeWidth="1" />
          </svg>
          {hover && (
            <div role="tooltip" style={{ position: "absolute", top: 0, left: hover.x, transform: "translateX(-50%)", pointerEvents: "none", background: "#fff", border: "1px solid #e9ebf4", borderRadius: 10, boxShadow: "0 8px 24px rgba(12,18,37,0.12)", padding: "8px 10px", fontSize: 12, minWidth: 130 }}>
              <div style={{ color: "#657090", marginBottom: 4 }}>{fmtDay(data[hover.i].label)}</div>
              {series.map((s, k) => (
                <div key={s.name} style={{ display: "flex", alignItems: "center", gap: 8, justifyContent: "space-between" }}>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "#657090" }}>
                    <span style={{ width: 10, height: 2, borderRadius: 1, background: SERIES[k] }} />{s.name}
                  </span>
                  <strong style={{ color: "#0f1222", fontVariantNumeric: "tabular-nums" }}>{format(data[hover.i].values[k])}</strong>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// BarList is a single-series horizontal bar list with the value at the tip.
export function BarList({ rows, format = compact }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return (
    <div style={{ display: "grid", gap: 10 }}>
      {rows.map((r) => (
        <div key={r.label} style={{ display: "grid", gap: 4 }} title={`${r.label}: ${format(r.value)}`}>
          <div className={styles.rowBetween} style={{ fontSize: 13, gap: 8 }}>
            <span style={{ overflowWrap: "anywhere" }}>{r.label}</span>
            <strong style={{ fontVariantNumeric: "tabular-nums" }}>{format(r.value)}</strong>
          </div>
          <div style={{ height: 8, borderRadius: 4, background: "#f1f3fb" }}>
            <div style={{ width: `${(r.value / max) * 100}%`, height: 8, borderRadius: 4, background: SERIES[0], minWidth: r.value > 0 ? 4 : 0 }} />
          </div>
        </div>
      ))}
    </div>
  );
}
