// @hey-api/openapi-ts configuration.
// Re-run after spec changes: cd packages/yunque-client && npm run generate
import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../../docs/openapi.yaml",
  output: {
    path: "src",
    format: "prettier",
  },
  plugins: [
    "@hey-api/client-fetch",
    "@hey-api/sdk",
    "@hey-api/typescript",
  ],
});
