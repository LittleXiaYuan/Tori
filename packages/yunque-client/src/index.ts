// Root entry stays intentionally explicit.
//
// Keep the package root limited to the generated OpenAPI surface plus the
// transport/bootstrap helpers. Focused product/runtime clients live behind
// explicit subpaths such as:
//   yunque-client/chat
//   yunque-client/packs
//   yunque-client/wasm-plugin
//   yunque-client/memory-time-travel
//   yunque-client/sbom-drift
//
// Do not re-export hand-written slices here; the package root should stay a
// thin contract boundary.
export * from './types.gen';
export * from './sdk.gen';
export { client, type CreateClientConfig } from './client.gen';
export {
  buildClientParams,
  createClient,
  createConfig,
  formDataBodySerializer,
  jsonBodySerializer,
  mergeHeaders,
  urlSearchParamsBodySerializer,
} from './client';
export type {
  Auth,
  Client,
  Config,
  OptionsLegacyParser,
  QuerySerializerOptions,
  RequestOptions,
  RequestResult,
  ResolvedRequestOptions,
  ResponseStyle,
  TDataShape,
} from './client';
