"use client";

import { useEffect, useState, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Spinner } from "@heroui/react";
import { useI18n } from "@/lib/i18n";
import { getAuthHeaders } from "@/lib/api-core";

const PUBLIC_PATHS = ["/login", "/setup"];
const AUTH_TIMEOUT_MS = 8000;
// Re-hit /v1/auth/status at most once every 5 minutes. This trades off
// "server-side token revocation is visible within 5 min" against "don't
// flood the backend on every route change".
const REVALIDATE_INTERVAL_MS = 5 * 60 * 1000;

export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { t } = useI18n();
  const [checking, setChecking] = useState(true);
  // Stores the wall-clock time of the last successful auth check; lets us
  // skip the network request on rapid route transitions while still picking
  // up revocations after the interval elapses.
  const lastCheckRef = useRef(0);

  useEffect(() => {
    let cancelled = false;
    let redirectFallback: ReturnType<typeof setTimeout> | undefined;
    const redirectToLogin = () => {
      router.replace("/login");
      redirectFallback = setTimeout(() => {
        if (typeof window === "undefined") return;
        if (!PUBLIC_PATHS.some((path) => window.location.pathname.startsWith(path))) {
          window.location.replace("/login");
        }
      }, 750);
    };
    const clearStoredCredentials = () => {
      localStorage.removeItem("yunque_token");
      localStorage.removeItem("yunque_api_key");
    };
    const bootstrapDesktopToken = async (signal: AbortSignal): Promise<Record<string, string>> => {
      const res = await fetch("/v1/auth/desktop-bootstrap", {
        method: "POST",
        signal,
      });
      if (!res.ok) return {};
      const data = await res.json().catch(() => ({}));
      const token = typeof data?.token === "string" ? data.token : "";
      if (!token) return {};
      localStorage.setItem("yunque_token", token);
      localStorage.removeItem("yunque_api_key");
      return { Authorization: `Bearer ${token}` };
    };

    if (PUBLIC_PATHS.some((path) => pathname?.startsWith(path))) {
      setChecking(false);
      return () => {
        cancelled = true;
        if (redirectFallback) clearTimeout(redirectFallback);
      };
    }

    // Cached result is still fresh — render immediately without a roundtrip.
    if (Date.now() - lastCheckRef.current < REVALIDATE_INTERVAL_MS) {
      setChecking(false);
      return;
    }

    const controller = new AbortController();
    const timeout = setTimeout(() => {
      controller.abort();
      if (!cancelled) setChecking(false);
    }, AUTH_TIMEOUT_MS);
    setChecking(true);

    (async () => {
      let headers = getAuthHeaders();
      if (!headers.Authorization && !headers["X-API-Key"]) {
        headers = await bootstrapDesktopToken(controller.signal);
      }
      if (!headers.Authorization && !headers["X-API-Key"]) {
        redirectToLogin();
        return null;
      }
      return fetch("/v1/auth/status", {
        headers,
        signal: controller.signal,
      });
    })()
      .then(async (res) => {
        if (cancelled) return null;
        if (res === null) return null;
        if (!res.ok) {
          // Surface status code to the catch branch via Error.cause so we
          // can tell "401 revoked" apart from "502 upstream hiccup".
          throw new Error(`HTTP ${res.status}`, { cause: res.status });
        }
        return res.json();
      })
      .then((data) => {
        if (cancelled || !data) return;
        if (data?.password_set === false) {
          // Server says no password is configured yet. Route to /login which
          // doubles as the "set password" flow; clear token so a stale one
          // can't sneak back.
          clearStoredCredentials();
          redirectToLogin();
          return;
        }
        if (!data?.authenticated) {
          clearStoredCredentials();
          redirectToLogin();
          return;
        }
        lastCheckRef.current = Date.now();
        setChecking(false);
      })
      .catch((error) => {
        if (cancelled) return;
        const status = (error as { cause?: unknown })?.cause;
        if (status === 401 || status === 403) {
          // Explicit auth failure → stored credentials are actually invalid/revoked.
          clearStoredCredentials();
          redirectToLogin();
          return;
        }
        // Network hiccup, timeout, 502/503, unreachable backend, etc.
        // Keep the token so users coming back on flaky connections aren't
        // kicked out. Render children optimistically; per-page API calls
        // will surface their own errors and the next navigation will retry.
        setChecking(false);
      })
      .finally(() => clearTimeout(timeout));

    return () => {
      cancelled = true;
      clearTimeout(timeout);
      if (redirectFallback) clearTimeout(redirectFallback);
      controller.abort();
    };
  }, [pathname, router]);

  if (PUBLIC_PATHS.some((path) => pathname?.startsWith(path))) {
    return <>{children}</>;
  }

  if (checking) {
    return (
      <div className="fixed inset-0 flex flex-col items-center justify-center gap-3" style={{ background: "var(--yunque-bg)" }}>
        <Spinner size="lg" />
        <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("auth.loading")}</div>
      </div>
    );
  }

  return <>{children}</>;
}
