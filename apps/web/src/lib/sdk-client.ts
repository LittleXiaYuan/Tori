import { BASE, getAuthHeaders } from "./api-core";

export interface YunqueSDKClientOptions {
  baseUrl: string;
  fetch: typeof fetch;
}

function browserOrigin(): string {
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return "http://localhost";
}

function toAbsoluteBaseUrl(base: string): string {
  const trimmed = base.trim();
  if (!trimmed) return browserOrigin();
  return new URL(trimmed, browserOrigin()).toString().replace(/\/+$/, "");
}

// Re-target every SDK request at the CURRENT backend base. SDK clients capture
// their baseUrl when constructed (often at module load, before the desktop
// backend port is resolved), so a client built against the default :9090 would
// otherwise keep hitting a dead port even after the real port is known. BASE is
// a live binding, so reading it per request always reflects the resolved
// backend (set by ensureApiBase in the desktop shell).
function routeToCurrentBackend(input: RequestInfo | URL): RequestInfo | URL {
  if (typeof Request !== "undefined" && input instanceof Request) return input;

  let url: URL;
  try {
    url = input instanceof URL ? input : new URL(String(input), browserOrigin());
  } catch {
    return input;
  }
  const base = BASE.trim();
  if (base) {
    const target = new URL(base, browserOrigin());
    url.protocol = target.protocol;
    url.host = target.host; // host includes the port
    return url.toString();
  }
  // No explicit base configured → prefer same-origin relative URLs.
  if (url.origin === browserOrigin()) return `${url.pathname}${url.search}${url.hash}`;
  return url.toString();
}

export const yunqueSDKFetch: typeof fetch = async (input, init) => {
  const headers = new Headers(init?.headers);
  for (const [key, value] of Object.entries(getAuthHeaders())) {
    if (!headers.has(key)) headers.set(key, value);
  }

  const response = await fetch(routeToCurrentBackend(input), {
    ...init,
    headers,
  });

  if (response.status === 401 && typeof window !== "undefined") {
    localStorage.removeItem("yunque_token");
  }

  return response;
};

export function createYunqueSDKClientOptions(): YunqueSDKClientOptions {
  return {
    baseUrl: toAbsoluteBaseUrl(BASE),
    fetch: yunqueSDKFetch,
  };
}
