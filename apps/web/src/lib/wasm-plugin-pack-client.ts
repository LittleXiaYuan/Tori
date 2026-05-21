import {
  createWASMPluginClient,
  type WASMPluginClient,
  type WASMPluginClientOptions,
} from "yunque-client/wasm-plugin";
import { createYunqueSDKClientOptions } from "./sdk-client";

// UI compatibility adapter only: constants, contracts, and HTTP transport live
// in yunque-client/wasm-plugin.
export * from "yunque-client/wasm-plugin";

export type {
  WASMPluginClient as WASMPluginPackClient,
  WASMPluginInstallRequest as WASMPluginInstallInput,
  WASMPluginRemoteInstallApprovalDecisionPlanRequest as WASMPluginRemoteInstallApprovalDecisionPlanInput,
  WASMPluginRemoteInstallApprovalPlanRequest as WASMPluginRemoteInstallApprovalPlanInput,
  WASMPluginRemoteInstallApprovalQueueWritebackRequest as WASMPluginRemoteInstallApprovalQueueWritebackInput,
  WASMPluginRemoteInstallApprovalWritebackPlanRequest as WASMPluginRemoteInstallApprovalWritebackPlanInput,
  WASMPluginRemoteInstallInstallerContinuationPlanRequest as WASMPluginRemoteInstallInstallerContinuationPlanInput,
  WASMPluginRemoteInstallInstallerDownloadWritebackRequest as WASMPluginRemoteInstallInstallerDownloadWritebackInput,
  WASMPluginRemoteInstallInstallerRegistrationPlanRequest as WASMPluginRemoteInstallInstallerRegistrationPlanInput,
  WASMPluginRemoteInstallPackageInspectWritebackRequest as WASMPluginRemoteInstallPackageInspectWritebackInput,
  WASMPluginRemoteInstallPlanRequest as WASMPluginRemoteInstallPlanInput,
  WASMPluginRemoteInstallSignatureVerificationWritebackRequest as WASMPluginRemoteInstallSignatureVerificationWritebackInput,
  WASMPluginStatusResponse as WASMPluginStatus,
} from "yunque-client/wasm-plugin";

export function createWASMPluginPackClient(
  options: Partial<WASMPluginClientOptions> = {},
): WASMPluginClient {
  return createWASMPluginClient({
    ...createYunqueSDKClientOptions(),
    ...options,
  });
}
