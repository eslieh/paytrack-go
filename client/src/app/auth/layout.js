// Sign-in, sign-up and recovery screens are thin, per-user pages: keep them out
// of search results but let crawlers follow their links (terms, privacy).
export const metadata = {
  title: "Sign in",
  robots: { index: false, follow: true },
};

export default function AuthLayout({ children }) {
  return (
    <div className="centered-screen">
      <div className="card">
        {children}
      </div>
    </div>
  );
}


