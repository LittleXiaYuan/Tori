import { buildWebChatEmbedSnippet, createWebChatEmbedClient } from "./webchat-embed";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertIncludes(actual: string, expected: string, message?: string): void { if (!actual.includes(expected)) throw new Error(message || `expected ${JSON.stringify(actual)} to include ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

test("WebChatEmbedClient builds escaped snippets with base defaults", () => {
  const client = createWebChatEmbedClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("") });
  const snippet = client.embedSnippet({ apiKey: "ya_123", title: "Tori <Assistant>", position: "bottom-left", theme: "dark", tenantId: "tenant-a" });
  assertIncludes(snippet, 'src="http://localhost:9090/v1/webchat/widget.js"'); assertIncludes(snippet, 'data-api-base="http://localhost:9090"'); assertIncludes(snippet, 'data-api-key="ya_123"'); assertIncludes(snippet, 'data-title="Tori &lt;Assistant&gt;"'); assertIncludes(snippet, 'data-position="bottom-left"'); assertIncludes(snippet, 'data-theme="dark"'); assertIncludes(snippet, 'data-tenant-id="tenant-a"');
});

test("buildWebChatEmbedSnippet supports custom script path and requires apiKey", () => {
  const snippet = buildWebChatEmbedSnippet({ apiKey: "key&quot;", scriptPath: "https://cdn.example/widget.js", apiBase: "https://api.example", placeholder: "Say \"hi\"" });
  assertIncludes(snippet, 'src="https://cdn.example/widget.js"'); assertIncludes(snippet, 'data-api-key="key&amp;quot;"'); assertIncludes(snippet, 'data-placeholder="Say &quot;hi&quot;"');
  try { buildWebChatEmbedSnippet({ apiKey: "" }); throw new Error("expected buildWebChatEmbedSnippet to reject"); } catch (error) { assert(error instanceof Error); assertEqual(error.message, "buildWebChatEmbedSnippet requires apiKey"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
