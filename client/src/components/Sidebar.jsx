"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { BookOpen, LogOut, Menu, X } from "lucide-react";
import styles from "./sidebar.module.css";

// Sidebar is the dashboard's left navigation. Below 860px it collapses into
// a top bar with a menu button that slides the panel in.
export default function Sidebar({ brand, items, activeHref, account, onSignOut }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();

  useEffect(() => setOpen(false), [pathname]);

  return (
    <>
      <header className={styles.mobileBar}>
        <button className={styles.menuBtn} onClick={() => setOpen((v) => !v)} aria-label={open ? "Close menu" : "Open menu"} aria-expanded={open}>
          {open ? <X size={18} /> : <Menu size={18} />}
        </button>
        <Link href="/dashboard" className={styles.brand}>{brand}</Link>
      </header>

      <aside className={`${styles.sidebar} ${open ? styles.open : ""}`}>
        <Link href="/dashboard" className={`${styles.brand} ${styles.sideBrand}`}>{brand}</Link>

        <nav className={styles.nav}>
          {items.map(({ href, label, icon: Icon }) => (
            <Link key={href} href={href} className={`${styles.link} ${activeHref === href ? styles.active : ""}`} aria-current={activeHref === href ? "page" : undefined}>
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
