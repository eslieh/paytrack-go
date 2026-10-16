"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname, useSearchParams } from "next/navigation";

// startTopLoader lets code that navigates programmatically (router.push after a
// form submit, for example) show the bar too.
export function startTopLoader() {
  if (typeof window !== "undefined") window.dispatchEvent(new Event("toploader:start"));
}

// TopLoader is the thin progress line at the top edge during page navigations.
// The App Router has no navigation events, so it starts on internal link clicks
// (or startTopLoader) and finishes when the URL actually changes.
export default function TopLoader() {
  const pathname = usePathname();
  const search = useSearchParams()?.toString();
  const [progress, setProgress] = useState(null); // null = hidden, 0-100 = visible
  const timers = useRef({});

  function clear() {
    clearInterval(timers.current.trickle);
    clearTimeout(timers.current.done);
    clearTimeout(timers.current.safety);
  }

  function start() {
    clear();
    setProgress(8);
    // Trickle towards 90%, slowing down as it goes, so it never looks stuck or "done" early.
    timers.current.trickle = setInterval(() => {
      setProgress((p) => (p == null ? p : Math.min(90, p + (90 - p) * 0.12)));
    }, 180);
    // Never get stuck: give up quietly after 10s.
    timers.current.safety = setTimeout(finish, 10000);
  }

  function finish() {
    clear();
    setProgress((p) => (p == null ? null : 100));
    timers.current.done = setTimeout(() => setProgress(null), 320);
  }

  // Start on clicks on internal links that will actually navigate.
  useEffect(() => {
    function onClick(e) {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const a = e.target.closest?.("a[href]");
      if (!a || a.target === "_blank" || a.hasAttribute("download") || a.getAttribute("aria-disabled") === "true") return;
      const url = new URL(a.href, location.href);
      if (url.origin !== location.origin) return;
      const sameDoc = url.pathname === location.pathname && url.search === location.search;
      if (sameDoc) return; // hash links and links to the current page don't navigate
      start();
    }
    const onStart = () => start();
    document.addEventListener("click", onClick, true);
    window.addEventListener("toploader:start", onStart);
    return () => {
      document.removeEventListener("click", onClick, true);
      window.removeEventListener("toploader:start", onStart);
      clear();
    };
  }, []);

  // The URL changed: the new page has rendered.
  useEffect(() => { finish(); }, [pathname, search]);

  if (progress == null) return null;
  return (
    <div className="toploader" role="progressbar" aria-label="Loading page" aria-valuenow={Math.round(progress)} aria-valuemin={0} aria-valuemax={100}>
      <div className="toploader-bar" style={{ transform: `scaleX(${progress / 100})`, opacity: progress >= 100 ? 0 : 1 }} />
    </div>
  );
}
