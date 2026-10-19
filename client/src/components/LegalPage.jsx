import Link from "next/link";
import styles from "./legal.module.css";
import { LEGAL, LEGAL_DOCS } from "@/lib/legal";
import BuiltBy from "./BuiltBy.jsx";

// LegalPage is the shared shell for /terms, /privacy and /disclaimer.
// `sections` is [{ id, title, body }] and drives both the TOC and the content.
export default function LegalPage({ current, title, kicker, tldr, sections }) {
  return (
    <div className={styles.page}>
      <header className={styles.top}>
        <Link href="/" className={styles.brand}>Paytrack <span>Legal</span></Link>
        <nav className={styles.docNav}>
          {LEGAL_DOCS.map((d) => (
            <Link key={d.href} href={d.href} className={d.href === current ? styles.docActive : ""} aria-current={d.href === current ? "page" : undefined}>
              {d.label}
            </Link>
          ))}
        </nav>
      </header>

      <div className={styles.layout}>
        <aside className={styles.toc}>
          <div className={styles.tocTitle}>On this page</div>
          {sections.map((s, i) => (
            <a key={s.id} href={`#${s.id}`}>
              <span className={styles.tocNum}>{String(i + 1).padStart(2, "0")}</span>
              {s.title}
            </a>
          ))}
        </aside>

        <main className={styles.main}>
          <div className={styles.kicker}>{kicker}</div>
          <h1>{title}</h1>
          <div className={styles.meta}>
            <code>version {LEGAL.version}</code>
            <span>Last updated {LEGAL.updated}</span>
            <span>{LEGAL.company}</span>
          </div>

          {tldr && (
            <div className={styles.tldr}>
              <div className={styles.tldrHead}>
                <span className={styles.dots}><i /><i /><i /></span>
                <span>tl;dr</span>
              </div>
              <ul>
                {tldr.map((t, i) => <li key={i}>{t}</li>)}
              </ul>
              <p className={styles.tldrNote}>The summary is a convenience. The full text below is what actually applies.</p>
            </div>
          )}

          {sections.map((s, i) => (
            <section key={s.id} id={s.id} className={styles.section}>
              <h2><span className={styles.num}>{String(i + 1).padStart(2, "0")}</span>{s.title}</h2>
              {s.body}
            </section>
          ))}

          <footer className={styles.footer}>
            <span style={{ display: "grid", gap: 6 }}>
              <BuiltBy />
              <span>Questions about this document: <a href={`mailto:${LEGAL.contact}`}>{LEGAL.contact}</a></span>
            </span>
            <span className={styles.footerLinks}>
              {LEGAL_DOCS.filter((d) => d.href !== current).map((d) => <Link key={d.href} href={d.href}>{d.label}</Link>)}
              <Link href="/docs">API docs</Link>
            </span>
          </footer>
        </main>
      </div>
    </div>
  );
}
