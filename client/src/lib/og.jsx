import { ImageResponse } from "next/og";
import { SITE } from "./site";

export const OG_SIZE = { width: 1200, height: 630 };

// The "P" mark used for the favicon, Apple icon and social images.
export function Mark({ size }) {
  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: size * 0.24,
        background: SITE.themeColor,
        color: "#fff",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        fontSize: size * 0.62,
        fontWeight: 800,
        letterSpacing: "-0.04em",
      }}
    >
      P
    </div>
  );
}

// ogImage renders a 1200x630 share card in the app's look: lavender
// background, navy code chip, indigo accent.
export function ogImage({ kicker, title, subtitle, code }) {
  const domain = SITE.url.replace(/^https?:\/\//, "");
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: "64px 72px",
          background: "linear-gradient(180deg, #e7e9ff 0%, #f6f7ff 55%, #ffffff 100%)",
          color: "#0f1222",
          fontFamily: "sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 18 }}>
          <Mark size={56} />
          <div style={{ fontSize: 38, fontWeight: 800, letterSpacing: "-0.03em" }}>{SITE.name}</div>
          {kicker && (
            <div style={{ marginLeft: 8, fontSize: 22, fontWeight: 700, color: "#19203a", background: "#eef1ff", border: "1px solid #dfe3fb", borderRadius: 999, padding: "6px 16px" }}>
              {kicker}
            </div>
          )}
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <div style={{ fontSize: 68, fontWeight: 800, lineHeight: 1.05, letterSpacing: "-0.035em", maxWidth: 1000 }}>{title}</div>
          <div style={{ fontSize: 30, lineHeight: 1.35, color: "#475467", maxWidth: 980 }}>{subtitle}</div>
        </div>

        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div
            style={{
              display: "flex",
              fontFamily: "monospace",
              fontSize: 24,
              color: "#dcf1ff",
              background: "#0f1222",
              border: "1px solid #1d2140",
              borderRadius: 14,
              padding: "14px 22px",
            }}
          >
            {code}
          </div>
          <div style={{ fontSize: 26, fontWeight: 600, color: SITE.themeColor }}>{domain}</div>
        </div>
      </div>
    ),
    OG_SIZE,
  );
}
