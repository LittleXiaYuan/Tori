import {
  createMemoryTimeTravelClient,
  type MemoryTimeTravelClient,
  type MemoryTimeTravelClientOptions,
} from "yunque-client/memory-time-travel";
import { createYunqueSDKClientOptions } from "./sdk-client";

// UI compatibility adapter only: Memory Time Travel contracts and transport
// are owned by yunque-client/memory-time-travel.
export * from "yunque-client/memory-time-travel";

export type {
  MemoryTimeTravelAuditVerificationResponse as MemoryTimeTravelAuditVerification,
  MemoryTimeTravelClient as MemoryTimeTravelPackClient,
  MemoryTimeTravelKVAuditLinksResponse as MemoryTimeTravelKVAuditLinksReport,
  MemoryTimeTravelStatusResponse as MemoryTimeTravelStatus,
} from "yunque-client/memory-time-travel";

export function createMemoryTimeTravelPackClient(
  options: Partial<MemoryTimeTravelClientOptions> = {},
): MemoryTimeTravelClient {
  return createMemoryTimeTravelClient({
    ...createYunqueSDKClientOptions(),
    ...options,
  });
}
