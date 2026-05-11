/** Lightweight skillhub-install SDK facade over the SkillHub slice. */
import {
  createSkillHubClient,
  SkillHubClient,
  SkillHubClientError,
  type SkillHubClientOptions,
  type SkillHubInstalledResponse,
  type SkillHubInstallResponse,
  type SkillHubRollbackResponse,
  type SkillHubUninstallResponse,
  type SkillHubUpdateResponse,
  type SkillHubUpdatesResponse,
} from "./skillhub.js";

export type {
  SkillHubClientOptions as SkillHubInstallClientOptions,
  SkillHubInstalledResponse,
  SkillHubInstallResponse,
  SkillHubRollbackResponse,
  SkillHubUninstallResponse,
  SkillHubUpdateResponse,
  SkillHubUpdatesResponse,
};

export { SkillHubClientError as SkillHubInstallClientError };

export class SkillHubInstallClient {
  private readonly client: SkillHubClient;

  constructor(options: SkillHubClientOptions) {
    this.client = createSkillHubClient(options);
  }

  installed(): Promise<SkillHubInstalledResponse> {
    return this.client.installed();
  }

  install(slug: string): Promise<SkillHubInstallResponse> {
    return this.client.install(slug);
  }

  uninstall(slug: string, method: "POST" | "DELETE" = "POST"): Promise<SkillHubUninstallResponse> {
    return this.client.uninstall(slug, method);
  }

  checkUpdates(): Promise<SkillHubUpdatesResponse> {
    return this.client.checkUpdates();
  }

  update(slug: string): Promise<SkillHubUpdateResponse> {
    return this.client.update(slug);
  }

  rollback(slug: string, version: string): Promise<SkillHubRollbackResponse> {
    return this.client.rollback(slug, version);
  }
}

export function createSkillHubInstallClient(options: SkillHubClientOptions): SkillHubInstallClient {
  return new SkillHubInstallClient(options);
}
