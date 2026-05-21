// ══════════════════════════════════════════════════════════════════════════
// Yunque Agent API Type Definitions
// ══════════════════════════════════════════════════════════════════════════
// Backwards-compatible barrel: the per-domain type modules now live under
// `./api-types/`. Existing consumers that import from "@/lib/api-types"
// keep working unchanged. New code should prefer the explicit module path
// (e.g. `@/lib/api-types/skills`) so tree-shaking and refactoring are
// easier to reason about.

export * from "./api-types/index";
