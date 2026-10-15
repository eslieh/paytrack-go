/** @type {import('next').NextConfig} */
const nextConfig = {
  async redirects() {
    // The wallet was removed with the move to the Go backend (payments settle to the tenant's own PayBill/Till).
    return [{ source: "/dashboard/wallet", destination: "/dashboard/payments", permanent: false }];
  },
};

export default nextConfig;
