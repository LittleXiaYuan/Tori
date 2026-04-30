/** @type {import('next').NextConfig} */
const isProd = process.env.NODE_ENV === "production";

// NOTE: `output: "export"` produces a static bundle and ignores `headers()`,
// so the CSP/X-Frame headers below only apply in `next dev` and when a custom
// server (or reverse proxy) forwards them in production. The canonical source
// of truth for production headers is the reverse proxy or the Go agent's
// embedded file server — keep them in sync with this list.
const securityHeaders = [
  {
    key: "Content-Security-Policy",
    value: [
      "default-src 'self'",
      // Next.js injects inline scripts for hydration and for our theme
      // bootstrap in layout.tsx; we allow self-hosted and inline, and pull
      // fonts from Google.
      "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
      "style-src 'self' 'unsafe-inline' fonts.googleapis.com",
      "font-src 'self' fonts.gstatic.com data:",
      // Images may legitimately come from user-uploaded theme backgrounds
      // (data:), favicons, or remote knowledge sources; narrow this further
      // if you disable those features.
      "img-src 'self' data: blob: https:",
      // Tauri 2 的 IPC 走 https://ipc.localhost 协议（Windows WebView2）
      // 以及 ipc: 协议（macOS/Linux）；不放行会导致前端 ti.invoke() 全部被 CSP 阻止。
      "connect-src 'self' http://localhost:* ws://localhost:* https://ipc.localhost ipc: https: wss:",
      "frame-ancestors 'none'",
      "base-uri 'self'",
      "form-action 'self'",
      "object-src 'none'",
    ].join("; "),
  },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
];

const nextConfig = {
  ...(isProd ? { output: "export" } : {}),
  images: { unoptimized: true },
  trailingSlash: true,
  // 关掉 dev 模式左下角的 "Rendering..."/编译状态浮标：放在桌面应用里
  // 看起来像应用 BUG，而 Tauri 已经有自己的窗口呈现机制。
  // 生产构建（output: export）本来就不会出现这个浮标，这里只影响 next dev。
  devIndicators: false,
  async rewrites() {
    if (isProd) return [];
    const api = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:9090";
    return [
      { source: "/v1/:path*", destination: `${api}/v1/:path*` },
      { source: "/api/:path*", destination: `${api}/api/:path*` },
      { source: "/healthz", destination: `${api}/healthz` },
    ];
  },
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
    ];
  },
};

module.exports = nextConfig;
