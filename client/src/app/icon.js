import { ImageResponse } from "next/og";
import { Mark } from "@/lib/og";

export const size = { width: 512, height: 512 };
export const contentType = "image/png";

export default function Icon() {
  return new ImageResponse(<Mark size={512} />, size);
}
