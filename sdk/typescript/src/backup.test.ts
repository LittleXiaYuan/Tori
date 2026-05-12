import { createBackupClient, BackupClientError } from "./backup";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BackupClient reads backup info with bearer token", async () => { const calls: { url: string; init?: RequestInit }[] = []; const client = createBackupClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: { "memory.json": 12 }, file_count: 1, total_bytes: 12, version: "dev" }); } }); const info = await client.info(); assertEqual(info.file_count, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/backup/info"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); });

test("BackupClient exports zip metadata", async () => { const client = createBackupClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response(new Blob(["zipdata"]), { status: 200, headers: { "Content-Type": "application/zip", "Content-Disposition": 'attachment; filename="yunque-backup-demo.zip"' } }) }); const exported = await client.export(); assertEqual(exported.filename, "yunque-backup-demo.zip"); assertEqual(exported.contentType, "application/zip"); assertEqual(await exported.blob.text(), "zipdata"); });

test("BackupClient imports zip as multipart form", async () => { const calls: { url: string; init?: RequestInit }[] = []; const client = createBackupClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ success: true, restored: 2, skipped: 0 }); } }); const imported = await client.import(new Blob(["zipdata"], { type: "application/zip" }), "restore.zip"); assertEqual(imported.restored, 2); assert(calls[0]?.init?.body instanceof FormData); assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), null); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); });

test("BackupClient throws BackupClientError", async () => { const client = createBackupClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "invalid backup" }, { status: 400 }) }); try { await client.info(); throw new Error("expected info to reject"); } catch (error) { assert(error instanceof BackupClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "invalid backup" }); assertEqual(error.message, "invalid backup"); } });
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
