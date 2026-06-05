import { formatErrorMessage } from "./error-utils";

let BASE = process.env.NEXT_PUBLIC_API_BASE || "";

let baseEnsured: Promise<void> | null = null;

// Desktop (Tauri) resolves the Go backend port at runtime (AGENT_ADDR /
// auto-pick), which can differ from the build-time NEXT_PUBLIC_API_BASE. Ask
// the Rust side for the live port so /v1 calls and the health probe always hit
// the running backend instead of a stale :9090. No-op outside the desktop shell.
export function ensureApiBase(): Promise<void> {
  if (baseEnsured) return baseEnsured;
  baseEnsured = (async () => {
    if (typeof window === "undefined") return;
    const invoke = (
      window as unknown as {
        __TAURI_INTERNALS__?: {
          invoke?: (cmd: string, args?: Record<string, unknown>) => Promise<unknown>;
        };
      }
    ).__TAURI_INTERNALS__?.invoke;
    if (!invoke) return;
    try {
      const port = await invoke("backend_port");
      if (typeof port === "number" && port > 0) {
        // Use `localhost` (not 127.0.0.1) so it matches the desktop's
        // capabilities.remote.urls (http://localhost:*) and the CSP
        // connect-src. The Rust side navigates the window to
        // http://localhost:{port}, so this keeps API calls same-origin and
        // avoids the CORS/CSP block that left the shell stuck "offline".
        BASE = `http://localhost:${port}`;
      }
    } catch {
      /* keep the build-time base */
    }
  })();
  return baseEnsured;
}

let apiKey = "";

export function setApiKey(key: string) {
  apiKey = key;
  if (typeof window !== "undefined") localStorage.setItem("yunque_api_key", key);
}

export function getApiKey(): string {
  if (apiKey) return apiKey;
  if (typeof window !== "undefined") {
    apiKey = localStorage.getItem("yunque_api_key") || "";
  }
  return apiKey;
}

export function getAuthHeaders(): Record<string, string> {
  const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") : "";
  if (token) return { Authorization: `Bearer ${token}` };
  const key = getApiKey();
  if (key) return { "X-API-Key": key };
  return {};
}

export async function fetcher<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
      ...opts?.headers,
    },
  });
  if (res.status === 401 && typeof window !== "undefined" && !path.includes("/auth/")) {
    const hadToken = !!localStorage.getItem("yunque_token");
    localStorage.removeItem("yunque_token");
    if (hadToken && !path.startsWith("/v1/") && !path.startsWith("/api/providers")) {
      window.location.href = "/login";
    }
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    const text = await res.text();
    let payload: unknown = text;
    try {
      payload = text ? JSON.parse(text) : text;
    } catch {
      // keep raw text
    }
    throw new Error(`${res.status}: ${formatErrorMessage(payload, text || "request failed")}`);
  }
  return res.json();
}

export { BASE };
