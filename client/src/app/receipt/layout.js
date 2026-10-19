// Receipts are private, per-payment pages: never index them.
export const metadata = {
  title: "Receipt",
  robots: { index: false, follow: false, nocache: true },
};

export default function ReceiptLayout({ children }) {
  return children;
}
