#!/usr/bin/env node
/**
 * Webview contract smoke check.
 *
 * The desktop webview talks to the Go backend through the next dev proxy. A few
 * contracts between those layers only break AT RUNTIME, INSIDE THE WEBVIEW, and
 * are invisible to a plain `curl` (which does not follow redirects by default):
 *
 *   - Trailing-slash redirect loops: next dev's 308 + path normalization can
 *     ping-pong into ERR_TOO_MANY_REDIRECTS. curl on a no-slash path sees a
 *     single 401 and looks fine; the webview spins forever.
 *   - IPv4 vs IPv6 binding: the backend binds 127.0.0.1; a "localhost" target
 *     can hit ::1 and fail as "Failed to fetch".
 *
 * This check reproduces what the webview does — follow redirects, counting hops
 * — against the trailing-slash variant, both direct-to-backend and through the
 * proxy, and FAILS LOUD if anything loops. Run it whenever the dev stack is up:
 *
 *   npm run check:webview              (from apps/desktop)
 *   FRONT=http://127.0.0.1:3001 BACKEND=http://127.0.0.1:9090 node scripts/check-webview-contract.mjs
 *
 * Exit code is non-zero on any hard failure so it can gate CI / pre-commit.
 */

const BACKEND = process.env.BACKEND || "http://127.0.0.1:9090";
const FRONT = process.env.FRONT || "http://127.0.0.1:3001";
const MAX_HOPS = 8;

const c = {
  ok: (m) => console.log(`\x1b[32m  ✓\x1b[0m ${m}`),
  bad: (m) => console.log(`\x1b[31m  ✗\x1b[0m ${m}`),
  info: (m) => console.log(`\x1b[36m[check]\x1b[0m ${m}`),
  warn: (m) => console.log(`\x1b[33m[check]\x1b[0m ${m}`),
};

/** Follow redirects manually, counting hops, so we can detect a loop. */
async function followCounting(url) {
  let current = url;
  let hops = 0;
  const chain = [];
  while (hops <= MAX_HOPS) {
    let res;
    try {
      res = await fetch(current, { redirect: "manual" });
    } catch (e) {
      return { error: e.message, chain };
    }
    chain.push(`${res.status} ${current}`);
    if (res.status >= 300 && res.status < 400) {
      const loc = res.headers.get("location");
      if (!loc) return { status: res.status, hops, chain };
      current = new URL(loc, current).toString();
      hops++;
      continue;
    }
    return { status: res.status, hops, chain };
  }
  return { looped: true, hops, chain };
}

let failures = 0;

/**
 * Assert that a URL resolves without a redirect loop. `mustReach` = the request
 * must connect at all (used for the backend, which must be up); when false a
 * connection error is reported as a skip rather than a failure (the proxy may
 * not be running in backend-only checks).
 */
async function assertNoLoop(label, url, { mustReach }) {
  const r = await followCounting(url);
  if (r.error) {
    if (mustReach) {
      c.bad(`${label}: cannot reach ${url} — ${r.error}`);
      failures++;
    } else {
      c.warn(`${label}: ${url} not reachable (${r.error}) — skipped`);
    }
    return;
  }
  if (r.looped) {
    c.bad(`${label}: REDIRECT LOOP (>${MAX_HOPS} hops) — this is the ERR_TOO_MANY_REDIRECTS bug`);
    r.chain.forEach((h) => console.log(`        ↳ ${h}`));
    failures++;
    return;
  }
  if (r.status >= 300 && r.status < 400) {
    c.bad(`${label}: dangling redirect (${r.status}) with no usable Location`);
    failures++;
    return;
  }
  c.ok(`${label}: ${r.status} in ${r.hops} redirect(s)`);
}

async function assertStatus(label, url, want) {
  try {
    const res = await fetch(url, { redirect: "manual" });
    if (res.status === want) {
      c.ok(`${label}: ${res.status}`);
    } else {
      c.bad(`${label}: got ${res.status}, want ${want}`);
      failures++;
    }
  } catch (e) {
    c.bad(`${label}: cannot reach ${url} — ${e.message}`);
    failures++;
  }
}

async function main() {
  c.info(`backend=${BACKEND}  front=${FRONT}`);

  // 1) Backend must be up and bound on IPv4 127.0.0.1.
  await assertStatus("backend /healthz (IPv4)", `${BACKEND}/healthz`, 200);

  // 2) Trailing-slash API path direct to backend must not loop.
  await assertNoLoop("backend /api/settings/schema/ (trailing slash)", `${BACKEND}/api/settings/schema/`, { mustReach: true });

  // 3) The real webview path: through the next dev proxy, trailing slash. This
  //    is the exact request shape that produced ERR_TOO_MANY_REDIRECTS.
  await assertNoLoop("proxy /api/settings/schema/ (trailing slash)", `${FRONT}/api/settings/schema/`, { mustReach: false });
  await assertNoLoop("proxy /api/settings/schema (no slash)", `${FRONT}/api/settings/schema`, { mustReach: false });

  console.log("");
  if (failures > 0) {
    c.bad(`${failures} contract check(s) FAILED`);
    process.exit(1);
  }
  c.ok("webview contract OK — no redirect loops, backend reachable on IPv4");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
