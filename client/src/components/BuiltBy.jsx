import { Github, Linkedin } from "lucide-react";
import { BUILDER } from "@/lib/legal";

// lucide's "X" is a close icon and "Twitter" is the old bird, so draw the X mark.
function XLogo({ size = 15 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

const ICONS = {
  GitHub: <Github size={16} aria-hidden="true" />,
  X: <XLogo />,
  LinkedIn: <Linkedin size={16} aria-hidden="true" />,
};

// BuiltBy is the footer credit shared by the landing page, docs and legal pages.
export default function BuiltBy() {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
      <span>© {new Date().getFullYear()} Paytrack · Built by {BUILDER.name}</span>
      <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
        {BUILDER.links.map((l) => (
          <a
            key={l.href}
            href={l.href}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`${BUILDER.name} on ${l.label}`}
            title={l.label}
            className="social-link"
          >
            {ICONS[l.label]}
          </a>
        ))}
      </span>
    </span>
  );
}
