"use client";

import Link from "next/link";
import { useLayoutEffect, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { BookOpen, LogOut, Menu, X } from "lucide-react";
import styles from "./sidebar.module.css";

// Sidebar is the dashboard's left navigation. Below 860px it collapses into
// a top bar with a menu button that slides the panel in.
// actions (e.g. the notification bell) sit next to the brand.
export default function Sidebar({ brand, brandHref = "/dashboard", items, activeHref, account, onSignOut, actions }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();
  const navRef = useRef(null);
  // pill: position of the sliding highlight behind the active link.
  const [pill, setPill] = useState(null);
  const placed = useRef(false);

  useEffect(() => setOpen(false), [pathname]);

  useLayoutEffect(() => {
    const el = navRef.current?.querySelector('[aria-current="page"]');
    if (!el) { setPill(null); return; }
    setPill({ top: el.offsetTop, height: el.offsetHeight, animate: placed.current });
    placed.current = true;
  }, [activeHref]);

  return (
    <>
      <header className={styles.mobileBar}>
        <button className={styles.menuBtn} onClick={() => setOpen((v) => !v)} aria-label={open ? "Close menu" : "Open menu"} aria-expanded={open}>
          {open ? <X size={18} /> : <Menu size={18} />}
        </button>
        <Link href={brandHref} className={styles.brand}>{brand}</Link>
        {actions && <div style={{ marginLeft: "auto" }}>{actions}</div>}
      </header>

      <aside className={`${styles.sidebar} ${open ? styles.open : ""}`}>
        <div className={styles.sideBrand} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
          <Link href={brandHref} className={styles.brand}>{brand}</Link>
          {actions}
        </div>

        <nav className={styles.nav} ref={navRef}>
          {pill && (
            <span
              className={styles.pill}
              style={{ transform: `translateY(${pill.top}px)`, height: pill.height, transition: pill.animate ? undefined : "none" }}
              aria-hidden="true"
            />
          )}
          {items.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              className={`${styles.link} ${activeHref === href ? styles.active : ""}`}
              aria-current={activeHref === href ? "page" : undefined}
              // Move the pill on click, before the route resolves, so it feels instant.
              onClick={(e) => setPill({ top: e.currentTarget.offsetTop, height: e.currentTarget.offsetHeight, animate: true })}
            >
              {Icon && <Icon size={18} />}
              <span>{label}</span>
            </Link>
          ))}
        </nav>

        <div className={styles.footer}>
          <Link href="/docs" target="_blank" className={styles.link}>
            <BookOpen size={18} />
            <span>API docs</span>
          </Link>
          {account && (
            <div className={styles.account}>
              <div className={styles.avatar}>{(account.name || account.email || "?").slice(0, 1).toUpperCase()}</div>
              <div className={styles.accountText}>
                <div className={styles.accountName}>{account.name}</div>
                <div className={styles.accountEmail}>{account.email}</div>
              </div>
              <button className={styles.signOut} onClick={onSignOut} title="Sign out" aria-label="Sign out">
                <LogOut size={16} />
              </button>
            </div>
          )}
        </div>
      </aside>

      {open && <div className={styles.scrim} onClick={() => setOpen(false)} />}
    </>
  );
}
