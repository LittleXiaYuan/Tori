/** @type {import('next').NextConfig} */
const isProd = process.env.NODE_ENV === "production";

const nextConfig = {
  ...(isProd ? { output: "export" } : {}),
  images: { unoptimized: true },
  trailingSlash: true,
  async rewrites() {
    if (isProd) return [];
    const api = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:9090";
    return [
      { source: "/v1/:path*", destination: `${api}/v1/:path*` },
      { source: "/api/:path*", destination: `${api}/api/:path*` },
      { source: "/healthz", destination: `${api}/healthz` },
    ];
  },
};

module.exports = nextConfig;
