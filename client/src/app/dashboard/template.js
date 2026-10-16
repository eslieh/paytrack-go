// Re-mounted on every dashboard navigation, so the entrance animation replays per page.
export default function DashboardTemplate({ children }) {
  return <div className="page-enter">{children}</div>;
}
