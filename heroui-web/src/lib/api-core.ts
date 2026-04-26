const BASE = process.env.NEXT_PUBLIC_API_BASE || "";

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
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json();
}

export { BASE };
