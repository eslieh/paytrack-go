// Re-mounted when moving between auth screens (sign in, sign up, reset...), so the
// card stays put and only its contents cross-fade.
export default function AuthTemplate({ children }) {
  return <div className="auth-enter">{children}</div>;
}
