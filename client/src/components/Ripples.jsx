"use client";

import { useEffect } from "react";

// Ripples adds a ripple from the pointer to any .btn that's pressed. It uses one
// delegated listener for the whole app, so buttons need no extra markup.
export default function Ripples() {
  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    function onDown(e) {
      const btn = e.target.closest?.(".btn");
      if (!btn || btn.disabled || btn.getAttribute("aria-disabled") === "true") return;
      const r = btn.getBoundingClientRect();
      const size = Math.max(r.width, r.height) * 2;
      const dot = document.createElement("span");
      dot.className = "ripple";
      dot.style.width = dot.style.height = `${size}px`;
      dot.style.left = `${e.clientX - r.left - size / 2}px`;
      dot.style.top = `${e.clientY - r.top - size / 2}px`;
      btn.appendChild(dot);
      dot.addEventListener("animationend", () => dot.remove(), { once: true });
    }
    document.addEventListener("pointerdown", onDown);
    return () => document.removeEventListener("pointerdown", onDown);
  }, []);
  return null;
}
