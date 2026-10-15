"use client";

import Link from "next/link";
import styles from "./tabs.module.css";

export default function Tabs({ items, activeHref, brand, actions }) {
  return (
    <div className={styles.tabs}>
      <Link href="/dashboard" className={styles.brand}>{brand}</Link>
      <nav className={styles.tablist}>
        {items.map((it) => (
          <Link
            key={it.href}
            href={it.href}
            className={`${styles.tab} ${activeHref === it.href ? styles.tabActive : ""}`}
          >
            {it.label}
          </Link>
        ))}
        {actions}
      </nav>
    </div>
  );
}
