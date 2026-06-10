import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetPackUIOriginCache, resolvePackUIOrigin } from "../pack-ui-origin";

describe("pack-ui-origin/resolvePackUIOrigin", () => {
  beforeEach(() => resetPackUIOriginCache());

  it("returns the isolation origin when the backend reports one", async () => {
    const fetchJSON = vi.fn(async () => ({ origin: "http://127.0.0.1:49152" }));
    await expect(resolvePackUIOrigin(fetchJSON as never)).resolves.toBe("http://127.0.0.1:49152");
  });

  it("falls back to same-origin ('') when disabled, invalid, or erroring", async () => {
    await expect(resolvePackUIOrigin((async () => ({ origin: "" })) as never)).resolves.toBe("");
    resetPackUIOriginCache();
    await expect(resolvePackUIOrigin((async () => ({ origin: "not-a-url" })) as never)).resolves.toBe("");
    resetPackUIOriginCache();
    await expect(resolvePackUIOrigin((async () => { throw new Error("404"); }) as never)).resolves.toBe("");
  });

  it("caches the probe for the session", async () => {
    const fetchJSON = vi.fn(async () => ({ origin: "http://127.0.0.1:1" }));
    await resolvePackUIOrigin(fetchJSON as never);
    await resolvePackUIOrigin(fetchJSON as never);
    expect(fetchJSON).toHaveBeenCalledTimes(1);
  });
});
