export const metadata = {
  title: {
    default: "Admin",
    template: "%s — Paytrack Admin",
  },
  robots: { index: false, follow: false },
};

export default function AdminLayout({ children }) {
  return children;
}
