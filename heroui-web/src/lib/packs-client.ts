import {
  createPacksClient as createSDKPacksClient,
  hasCatalogSourceIssues,
  summarizeCapabilityPrepare,
  summarizeCatalogSourceReports,
  type PackMutationResponse,
  type PackPruneResponse,
  type PacksClient as SDKPacksClient,
  type PacksClientOptions,
} from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "./sdk-client";

export {
  hasCatalogSourceIssues,
  summarizeCapabilityPrepare,
  summarizeCatalogSourceReports,
};

export type * from "yunque-client/packs";
export type PacksPruneResponse = PackPruneResponse;

export type PacksClient = SDKPacksClient & {
  installLocal(
    manifestPath: string,
    source?: string,
    download?: boolean,
  ): Promise<PackMutationResponse>;
  installFromURL(
    manifestUrl: string,
    source?: string,
    download?: boolean,
  ): Promise<PackMutationResponse>;
};

export function createPacksClient(
  options: Partial<PacksClientOptions> = {},
): PacksClient {
  const client = createSDKPacksClient({
    ...createYunqueSDKClientOptions(),
    ...options,
  }) as PacksClient;

  client.installLocal = (manifestPath, source, download) =>
    client.install({ manifestPath, source, download });
  client.installFromURL = (manifestUrl, source, download) =>
    client.install({ manifestUrl, source, download });

  return client;
}
