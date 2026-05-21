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

function normalizeSameOriginSDKUrl(input: RequestInfo | URL): RequestInfo | URL {
  if (BASE) return input;
  if (typeof Request !== "undefined" && input instanceof Request) return input;

  let url: URL;
  try {
    url = input instanceof URL ? input : new URL(String(input));
  } catch {
    return input;
  }
  if (url.origin !== browserOrigin()) return input;
  return `${url.pathname}${url.search}${url.hash}`;
}

export const yunqueSDKFetch: typeof fetch = async (input, init) => {
  const headers = new Headers(init?.headers);
  for (const [key, value] of Object.entries(getAuthHeaders())) {
    if (!headers.has(key)) headers.set(key, value);
  }

  const response = await fetch(normalizeSameOriginSDKUrl(input), {
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
