/** @type {import('next').NextConfig} */

// The dashboard talks to the PayTrack API through its own /api path, so the
// session (refresh) cookie is first-party on every dashboard domain.
const API_PROXY_TARGET = (process.env.API_PROXY_TARGET || "http://localhost:9710").replace(/\/+$/, "");

const nextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_PROXY_TARGET}/:path*` }];
  },
  async redirects() {
    // The wallet was removed with the move to the Go backend (payments settle to the tenant's own PayBill/Till).
    return [{ source: "/dashboard/wallet", destination: "/dashboard/payments", permanent: false }];
  },
};

export default nextConfig;
