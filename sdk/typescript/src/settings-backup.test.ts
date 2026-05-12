import { createSettingsBackupClient, SettingsBackupClientError } from "./settings-backup";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SettingsBackupClient reads backup info with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsBackupClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: { "config/.env": 12 }, file_count: 1, total_bytes: 12 }); } });
  assertEqual((await client.backupInfo()).file_count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/backup/info");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SettingsBackupClient exports and imports backup with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsBackupClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/export")) return new Response(new Blob(["zipdata"]), { status: 200, headers: { "Content-Type": "application/zip", "Content-Disposition": 'attachment; filename="yunque-backup.zip"' } }); return jsonResponse({ success: true, restored: 2, skipped: 0 }); } });
  const exported = await client.exportBackup();
  const imported = await client.importBackup(new Blob(["zipdata"], { type: "application/zip" }), "restore.zip");
  assertEqual(exported.filename, "yunque-backup.zip");
  assertEqual(exported.contentType, "application/zip");
  assertEqual(imported.restored, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/backup/export");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/backup/import");
  assert(calls[1]?.init?.body instanceof FormData);
  assertEqual(new Headers(calls[1]?.init?.headers).get("content-type"), null, "multipart boundary must be set by fetch");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SettingsBackupClient exposes nested backup errors", async () => {
  const client = createSettingsBackupClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SETTINGS_BACKUP", message: "backup failed" } }, { status: 500 }) });
  try { await client.backupInfo(); throw new Error("expected backupInfo to reject"); } catch (error) { assert(error instanceof SettingsBackupClientError); assertEqual(error.name, "SettingsClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SETTINGS_BACKUP", message: "backup failed" } }); assertEqual(error.message, "backup failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
