// Browser-side URL/navigation safety helpers.
//
// Two common pitfalls we defend against across the app:
//  1. `window.open(url, "_blank")` without `noopener,noreferrer` — the popup
//     can navigate the opener via `window.opener.location` (reverse tabnabbing).
//  2. URLs that originate from untrusted places (model output, localStorage,
//     backend echoing user input) used as the navigation target. A
//     `javascript:` or `data:text/html` here is a same-origin XSS vector.
//
// Centralising the behaviour here makes audit greppable (`openExternal(`)
// and ensures every new caller opts in by default rather than repeating the
// defensive boilerplate.

const SAFE_NAV_SCHEMES = new Set(["http:", "https:", "mailto:"]);

/**
 * Return true only if the URL is safe to navigate to from a click handler.
 * Blank/null, `javascript:`, `data:`, `vbscript:`, `file:` are rejected.
 */
export function isSafeNavURL(raw: string | null | undefined): boolean {
  if (!raw) return false;
  const trimmed = String(raw).trim();
  if (!trimmed) return false;
  // Block obvious XSS-by-URL vectors up front. `new URL` would happily parse
  // `javascript:alert(1)` and expose a `protocol` of `javascript:`.
  try {
    const u = new URL(trimmed, typeof window !== "undefined" ? window.location.origin : "http://localhost/");
    return SAFE_NAV_SCHEMES.has(u.protocol);
  } catch {
    // Relative URLs without a base also fall through here; we treat them as
    // unsafe in the "open external" code path because callers should have
    // fully-qualified URLs at that point.
    return false;
  }
}

/**
 * Safely open an external URL in a new tab. Rejects unsafe schemes silently
 * (callers can check the boolean return value for UX feedback) and always
 * adds `noopener,noreferrer`.
 */
export function openExternal(url: string | null | undefined): boolean {
  if (!isSafeNavURL(url)) return false;
  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- isSafeNavURL guarantees url is a non-empty string
  window.open(url!, "_blank", "noopener,noreferrer");
  return true;
}

/**
 * Validate an asset URL stored in client-side preferences (theme bg, favicon,
 * etc.). Only `https:` and `data:image/*` are allowed — no `http:` so we
 * never leak the referrer on an insecure transport, and no plain `data:`
 * that could embed HTML.
 */
export function isSafeAssetURL(raw: string | null | undefined): boolean {
  if (!raw) return false;
  const trimmed = String(raw).trim();
  if (!trimmed) return false;
  if (trimmed.startsWith("data:image/")) return true;
  try {
    const u = new URL(trimmed);
    return u.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Validate an API base URL candidate before it is persisted into the
 * apiClient's global `BASE`. Rejects anything that is not an http(s) URL or
 * does not look like a pure origin (we do not allow embedded paths, which
 * would subtly shift where every fetch lands).
 */
export function isSafeApiBase(raw: string | null | undefined): boolean {
  if (!raw) return false;
  const trimmed = String(raw).trim();
  if (!trimmed) return false;
  try {
    const u = new URL(trimmed);
    if (u.protocol !== "http:" && u.protocol !== "https:") return false;
    if (u.search || u.hash) return false;
    return true;
  } catch {
    return false;
  }
}
