import { defineConfig } from "vitest/config";
import path from "node:path";

// Vitest is intentionally scoped to pure-function tests right now. The
// jsdom / happy-dom environments and @testing-library setup will land in a
// follow-up PR; today's goal is just to establish the runner and let CI
// start catching regressions in the chat / slash-command helpers that were
// extracted out of chat/page.tsx (see TECH-DEBT-2026-04-18.md §7).
export default defineConfig({
  test: {
    environment: "node",
    globals: false,
    include: [
      "src/**/*.{test,spec}.ts",
      "src/**/*.{test,spec}.tsx",
      "src/**/__tests__/**/*.{ts,tsx}",
    ],
  },
  resolve: {
    // Mirror the `@/*` path alias from tsconfig.json so tests can import
    // the same specifiers as app code.
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
