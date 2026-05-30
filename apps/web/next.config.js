/** @type {import('next').NextConfig} */
const path = require("path");
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
      "connect-src 'self' http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:* http://ipc.localhost https://ipc.localhost ipc: https: wss:",
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
  // next dev 会把带尾斜杠的请求 308 重定向到无斜杠变体。桌面 webview 经
  // SPA 路由发出的 /api/*、/v1/* 代理请求带了尾斜杠，于是 308 与 webview
  // 的归一化互相弹跳，浏览器报 ERR_TOO_MANY_REDIRECTS（命令行 curl 手测无
  // 斜杠路径看不到，这是 webview 独有的死循环）。关掉这个运行时 308 即可，
  // 静态导出（output:export）的文件名行为不受影响。
  skipTrailingSlashRedirect: true,
  images: { unoptimized: true },
  // Keep the local SDK as a first-class source dependency during Turbopack
  // builds so subpath imports like `yunque-client/packs` resolve directly to
  // the shared TS source under packages/yunque-client/src.
  transpilePackages: ["yunque-client"],
  turbopack: {
    // Turbopack needs the repo root as its workspace root to see the linked
    // package outside apps/web/.
    root: path.resolve(__dirname, "..", ".."),
    resolveAlias: {
      "yunque-client": path.resolve(__dirname, "..", "..", "packages", "yunque-client", "src"),
    },
  },
  experimental: {
    externalDir: true,
  },
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  // 关掉 dev 模式左下角的 "Rendering..."/编译状态浮标：放在桌面应用里
  // 看起来像应用 BUG，而 Tauri 已经有自己的窗口呈现机制。
  // 生产构建（output: export）本来就不会出现这个浮标，这里只影响 next dev。
  devIndicators: false,
  ...(!isProd
    ? {
        async rewrites() {
          // Use 127.0.0.1 (not "localhost") as the default proxy target: on
          // Windows "localhost" resolves to IPv6 ::1 first, but the agent binds
          // IPv4 127.0.0.1:9090. A "localhost" target intermittently hits ::1
          // and fails with ECONNREFUSED, surfacing as "Failed to fetch" in the
          // desktop webview. 127.0.0.1 targets the IPv4 listener deterministically.
          const api = process.env.NEXT_PUBLIC_API_BASE || "http://127.0.0.1:9090";
          return {
            beforeFiles: [
              { source: "/v1/:path*", destination: `${api}/v1/:path*` },
              { source: "/api/:path*", destination: `${api}/api/:path*` },
              { source: "/healthz", destination: `${api}/healthz` },
            ],
          };
        },
        async headers() {
          return [
            { source: "/:path*", headers: securityHeaders },
          ];
        },
      }
    : {}),
};

module.exports = nextConfig;
