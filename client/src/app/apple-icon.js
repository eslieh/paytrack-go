import { ImageResponse } from "next/og";
import { SITE } from "@/lib/site";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";

// iOS adds its own rounded mask, so this one is a full square.
export default function AppleIcon() {
  return new ImageResponse(
    (
      <div style={{ width: "100%", height: "100%", background: SITE.themeColor, color: "#fff", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 116, fontWeight: 800 }}>
        P
      </div>
    ),
    size,
  );
}
