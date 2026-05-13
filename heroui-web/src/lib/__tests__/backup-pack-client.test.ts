import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createBackupPackClient } from "../backup-pack-client";
import { setApiKey } from "../api-core";

beforeEach(() => {
  setApiKey("");
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("backup-pack-client", () => {
  it("reads backup info through the backup pack route", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ files: { "a.db": 10 }, file_count: 1, total_bytes: 10, version: "dev" }), { status: 200 }),
    );

    const info = await createBackupPackClient().info();

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/backup/info");
    expect(info.file_count).toBe(1);
  });

  it("exports backup with api key and downloaded filename", async () => {
    setApiKey("key-1");
    const click = vi.fn();
    const createElement = vi.spyOn(document, "createElement").mockReturnValue({ click } as unknown as HTMLAnchorElement);
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn() });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    const createObjectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:backup");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(new Blob(["zip"]), { status: 200, headers: { "Content-Disposition": 'attachment; filename="backup.zip"' } }),
    );

    await createBackupPackClient().export();

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/backup/export");
    expect((spy.mock.calls[0]?.[1] as RequestInit).headers).toEqual({ "X-API-Key": "key-1" });
    expect(createElement).toHaveBeenCalledWith("a");
    expect(createObjectURL).toHaveBeenCalledOnce();
    expect(click).toHaveBeenCalledOnce();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:backup");
  });

  it("imports backup as form data", async () => {
    const file = new File(["zip"], "backup.zip", { type: "application/zip" });
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "ok", files_restored: 2, from_version: "dev", size_bytes: 3 }), { status: 200 }),
    );

    const result = await createBackupPackClient().import(file);

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/backup/import");
    const init = spy.mock.calls[0]?.[1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
    expect(result.files_restored).toBe(2);
  });
});
