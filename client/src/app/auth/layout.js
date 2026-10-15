export const metadata = {
  title: "Paytrack • Auth",
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


