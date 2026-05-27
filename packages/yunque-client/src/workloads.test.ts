import { buildWorkloadCatalogHref, formatWorkloadCapabilities, getWorkloadPresetById, listWorkloadPresets, WORKLOAD_PRESETS } from "./workloads";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => void }> = [];
function test(name: string, fn: () => void): void { tests.push({ name, fn }); }

test("workload presets are shared SDK metadata", () => {
  assertEqual(WORKLOAD_PRESETS.length, 5);
  const browser = getWorkloadPresetById("browser-rpa");
  assert(browser);
  assertEqual(formatWorkloadCapabilities(browser), "browser.intent.plan, rpa.replay.dry_run, rpa.executor.plan");
  assertEqual(buildWorkloadCatalogHref(browser), "/packs?preset=browser-rpa");
  assertEqual(getWorkloadPresetById("missing"), undefined);
});

test("listWorkloadPresets returns defensive copies", () => {
  const first = listWorkloadPresets()[0];
  assert(first);
  first.capabilities.push("mutated.capability");
  assert(!WORKLOAD_PRESETS[0]?.capabilities.includes("mutated.capability"));
});

let failures = 0; for (const { name, fn } of tests) { try { fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
