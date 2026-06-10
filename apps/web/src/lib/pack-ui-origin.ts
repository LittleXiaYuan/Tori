import { fetcher } from "@/lib/api-core";

// Resolves the dedicated pack-UI isolation listener origin (PACK_UI_ADDR).
// "" means disabled (or older backend): the DLC host falls back to serving
// bundles same-origin from the main listener. Cached for the session.

type FetchJSON = <T>(path: string) => Promise<T>;

let cached: Promise<string> | null = null;

export function resolvePackUIOrigin(fetchJSON: FetchJSON = fetcher): Promise<string> {
  if (!cached) {
    cached = fetchJSON<{ origin?: string }>("/v1/packs/ui-origin")
      .then((r) => (typeof r?.origin === "string" && r.origin.startsWith("http") ? r.origin : ""))
      .catch(() => "");
  }
  return cached;
}

/** Test hook: clear the session cache. */
export function resetPackUIOriginCache(): void {
  cached = null;
}
