import type { PackInstallRequest, PackManifest } from "yunque-client/packs";

export type PackPermissionGroupKey =
  | "read"
  | "write"
  | "network"
  | "browser"
  | "computer"
  | "sandbox"
  | "memoryKnowledge"
  | "admin"
  | "other";

export type PackPermissionGroup = {
  key: PackPermissionGroupKey;
  label: string;
  description: string;
  permissions: string[];
};

export type PackRiskProfile = {
  level: "low" | "medium" | "high";
  label: string;
  description: string;
  requiresAuthorization: boolean;
};

export type PackFeatureFlags = {
  hasFrontend: boolean;
  hasBackend: boolean;
  hasWasm: boolean;
  isIframeBundle: boolean;
  hasSdk: boolean;
  canRollback: boolean;
};

export type PackCatalogActionKind = "install" | "enable" | "update" | "use";

export type PackCatalogAction = {
  kind: PackCatalogActionKind;
  label: string;
  disabled: boolean;
  needsInstallSource: boolean;
};

export type PackUsability = {
  kind: "actionable" | "experimental" | "infrastructure" | "documented";
  label: string;
  description: string;
  primaryActionLabel?: string;
  primaryActionPath?: string;
  limitation?: string;
};

export type PackReadiness = {
  level: "complete" | "needs_context" | "needs_entry";
  label: string;
  description: string;
  missing: string[];
};

type EntryLike = {
  manifest: PackManifest;
  package_url?: string;
  manifest_path?: string;
  manifest_url?: string;
  source?: string;
  release_url?: string;
  downloadable?: boolean;
  sha256?: string;
  installed?: boolean;
  enabled?: boolean;
  update_action?: string;
};

const PERMISSION_GROUPS: Array<Omit<PackPermissionGroup, "permissions"> & { matches: RegExp[] }> = [
  {
    key: "computer",
    label: "电脑使用",
    description: "涉及桌面、窗口、文件、输入设备或本机/云桌面控制的能力。",
    matches: [/(computer|desktop|window|screen|keyboard|mouse|local|cloud-computer)/i],
  },
  {
    key: "browser",
    label: "浏览器",
    description: "读取或规划浏览器页面、标签页、截图、意图与浏览器自动化。",
    matches: [/(browser|tab|dom|page|screenshot|intent|rpa)/i],
  },
  {
    key: "sandbox",
    label: "沙箱",
    description: "在隔离环境中运行插件、脚本、WASM 或临时执行器。",
    matches: [/(sandbox|wasm|plugin|execute|runtime|executor)/i],
  },
  {
    key: "memoryKnowledge",
    label: "记忆/知识",
    description: "读取、写入或整理云雀的记忆、知识库、经验和上下文。",
    matches: [/(memory|knowledge|experience|cogni|context|persona|recall|learn)/i],
  },
  {
    key: "admin",
    label: "管理能力",
    description: "管理模型、租户、审计、Workers、审批、能力包或控制面资源。",
    matches: [/(admin|control|tenant|worker|audit|metric|model|approval|trust|pack|rbac|role|permission|manage)/i],
  },
  {
    key: "read",
    label: "读取",
    description: "读取状态、配置、列表、历史或结果，不直接修改数据。",
    matches: [/(^|:)(read|list|get|view|inspect|status|observe|search|query|stats|history|detail|info)(:|$)/i],
  },
  {
    key: "write",
    label: "写入",
    description: "创建、更新、删除、导入、上传或写回数据。",
    matches: [/(^|:)(write|create|update|delete|remove|import|upload|ingest|sync|apply|decision|approve|queue)(:|$)/i],
  },
  {
    key: "network",
    label: "联网",
    description: "访问远程地址、下载资源、连接外部服务或订阅网络事件。",
    matches: [/(network|http|url|remote|download|connect|webhook|events:subscribe|provider|connector|repo|github|oss)/i],
  },
];

const HIGH_RISK_PACK_IDS = new Set([
  "yunque.pack.computer-use",
  "yunque.pack.wasm-plugin",
  "yunque.pack.browser-intent",
]);

export function packExamples(manifest: PackManifest, limit = 3): string[] {
  const metadata = manifest.metadata || {};
  return ["example1", "example2", "example3", "example4", "example5"]
    .map((key) => metadata[key])
    .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
    .slice(0, limit);
}

export function packUsability(manifest: PackManifest): PackUsability {
  const metadata = manifest.metadata || {};
  const declared = metadata.usability;
  const primaryActionLabel = metadata.primaryActionLabel;
  const primaryActionPath = metadata.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
  const limitation = metadata.limitation;

  if (declared === "experimental" || manifest.status === "alpha") {
    return {
      kind: "experimental",
      label: "实验能力",
      description: "可以体验，但不要把它当成稳定主路径；请先看当前限制。",
      primaryActionLabel,
      primaryActionPath,
      limitation,
    };
  }
  if (declared === "infrastructure") {
    return {
      kind: "infrastructure",
      label: "后台支撑",
      description: "它主要被云雀内部、任务或其他页面调用，通常不需要单独打开。",
      primaryActionLabel,
      primaryActionPath,
      limitation,
    };
  }
  if (declared === "actionable" || primaryActionPath) {
    return {
      kind: "actionable",
      label: "可直接使用",
      description: "启用后可以打开对应入口查看、编辑或执行相关工作。",
      primaryActionLabel,
      primaryActionPath,
      limitation,
    };
  }
  return {
    kind: "documented",
    label: "能力声明",
    description: "当前主要提供能力声明、权限和运行时信息，具体使用由云雀自动调度。",
    primaryActionLabel,
    primaryActionPath,
    limitation,
  };
}

export function packReadiness(manifest: PackManifest): PackReadiness {
  const metadata = manifest.metadata || {};
  const frontend = manifest.frontend || {};
  const backend = manifest.backend || {};
  const examples = packExamples(manifest, 1);
  const usageSurface = typeof metadata.usageSurface === "string" && metadata.usageSurface.trim().length > 0;
  const primaryActionPath = typeof metadata.primaryActionPath === "string" && metadata.primaryActionPath.trim().length > 0;
  const hasFrontend = (frontend.menus?.length ?? 0) > 0 || (frontend.routes?.length ?? 0) > 0 || Boolean(frontend.assets?.entry);
  const hasBackend =
    (backend.capabilities?.length ?? 0) > 0 ||
    (backend.routes?.length ?? 0) > 0 ||
    (backend.routeSpecs?.length ?? 0) > 0;
  const missing: string[] = [];

  if (examples.length === 0) missing.push("使用示例");
  if (!usageSurface) missing.push("用户感知位置");
  if (!primaryActionPath && !hasFrontend) missing.push("打开/使用入口");
  if (!hasBackend) missing.push("后端能力声明");

  if (missing.length === 0) {
    return {
      level: "complete",
      label: "说明完整",
      description: "用途、入口、示例和能力边界都已声明，用户更容易判断是否需要安装。",
      missing,
    };
  }

  if (missing.includes("打开/使用入口") || missing.includes("后端能力声明")) {
    return {
      level: "needs_entry",
      label: "需补入口",
      description: "这个能力包还缺用户入口或后端能力声明，适合先用小羽补齐可用路径。",
      missing,
    };
  }

  return {
    level: "needs_context",
    label: "需补说明",
    description: "能力本体存在，但还需要补齐用户在哪里感知、如何使用或典型场景。",
    missing,
  };
}

export function packFeatureFlags(manifest: PackManifest): PackFeatureFlags {
  const frontend = manifest.frontend || {};
  const backend = manifest.backend || {};
  const assetsType = String(frontend.assets?.type || "").toLowerCase();
  const capabilities = backend.capabilities || [];
  const permissions = backend.permissions || [];
  const sdkEntries = Object.values(manifest.sdk || {}).filter((value) => typeof value === "string" && value.trim().length > 0);
  const haystack = [
    manifest.id,
    assetsType,
    ...capabilities,
    ...permissions,
    ...(backend.routes || []),
    ...(backend.routeSpecs || []).map((route) => `${route.method} ${route.path}`),
  ].join(" ").toLowerCase();

  return {
    hasFrontend: (frontend.menus?.length ?? 0) > 0 || (frontend.routes?.length ?? 0) > 0 || Boolean(frontend.assets?.entry),
    hasBackend: capabilities.length > 0 || (backend.routes?.length ?? 0) > 0 || (backend.routeSpecs?.length ?? 0) > 0,
    hasWasm: haystack.includes("wasm"),
    isIframeBundle: assetsType === "iframe-bundle",
    hasSdk: sdkEntries.length > 0,
    canRollback: Boolean(manifest.update?.rollback),
  };
}

export function groupPackPermissions(permissions: readonly string[] = []): PackPermissionGroup[] {
  const grouped = new Map<PackPermissionGroupKey, PackPermissionGroup>();
  const ensureGroup = (key: PackPermissionGroupKey): PackPermissionGroup => {
    const found = grouped.get(key);
    if (found) return found;
    const definition = PERMISSION_GROUPS.find((group) => group.key === key);
    const group = definition
      ? { key: definition.key, label: definition.label, description: definition.description, permissions: [] }
      : { key: "other" as const, label: "其他", description: "暂未归类的能力声明，会按原始权限名展示。", permissions: [] };
    grouped.set(key, group);
    return group;
  };

  for (const permission of permissions) {
    const matched = PERMISSION_GROUPS.find((group) => group.matches.some((pattern) => pattern.test(permission)));
    ensureGroup(matched?.key || "other").permissions.push(permission);
  }

  return [...grouped.values()].filter((group) => group.permissions.length > 0);
}

export function riskProfileForPack(manifest: PackManifest): PackRiskProfile {
  const permissions = manifest.backend?.permissions || [];
  const groups = groupPackPermissions(permissions).map((group) => group.key);
  const flags = packFeatureFlags(manifest);
  const highRisk =
    HIGH_RISK_PACK_IDS.has(manifest.id) ||
    groups.includes("computer") ||
    permissions.some((permission) => /(finance|financial|trade|trading|broker|order)/i.test(permission)) ||
    (groups.includes("browser") && groups.includes("write")) ||
    (flags.hasWasm && permissions.some((permission) => /(execute|write|download|remote|network)/i.test(permission)));

  if (highRisk) {
    return {
      level: "high",
      label: "需要授权",
      description: "涉及浏览器、电脑使用、WASM 执行或远程下载等敏感能力，启用前建议确认来源与权限。",
      requiresAuthorization: true,
    };
  }

  const mediumRisk =
    groups.includes("write") ||
    groups.includes("network") ||
    groups.includes("sandbox") ||
    groups.includes("admin") ||
    groups.includes("memoryKnowledge");
  if (mediumRisk) {
    return {
      level: "medium",
      label: "需留意",
      description: "会读取或修改部分数据/外部资源，可随时禁用或回滚。",
      requiresAuthorization: false,
    };
  }

  return {
    level: "low",
    label: "低风险",
    description: "主要读取状态或提供界面入口，不会默认执行高危操作。",
    requiresAuthorization: false,
  };
}

export function packUsageExplanation(manifest: PackManifest): string[] {
  const usability = packUsability(manifest);
  const metadata = manifest.metadata || {};
  if (manifest.id === "yunque.pack.computer-use") {
    return [
      "启用后 Planner 可生成电脑使用计划。",
      "当前阶段只做计划、权限与人工确认提示，不执行本机控制。",
      "后续真实浏览器动作、云桌面或本机桌面控制会继续走授权门禁。",
    ];
  }

  const capabilities = manifest.backend?.capabilities || [];
  const hasSkill = capabilities.some((capability) => /skill|planner|agent|task|workflow/i.test(capability));
  const hasContext = capabilities.some((capability) => /context|memory|knowledge|recall|persona|experience/i.test(capability));
  const hasFrontend = packFeatureFlags(manifest).hasFrontend;
  const explanation: string[] = [];

  if (capabilities.length > 0) {
    explanation.push("云雀会把它声明的能力加入可选择工具集，按任务需要启用。");
  }
  if (hasSkill) {
    explanation.push("它可以作为 SkillProvider，为规划、任务或自动化流程补充专用技能。");
  }
  if (hasContext) {
    explanation.push("它可以作为 ContextProvider，为对话和任务补充记忆、知识或经验上下文。");
  }
  if (hasFrontend) {
    explanation.push("启用后会同步它声明的界面入口，可从能力包中心、侧栏或命令菜单打开。");
  }
  if (usability.primaryActionPath) {
    explanation.push(`${usability.primaryActionLabel || "主要入口"}：${usability.primaryActionPath}`);
  }
  if (typeof metadata.usageSurface === "string" && metadata.usageSurface.trim().length > 0) {
    explanation.push(`用户能感知到的位置：${metadata.usageSurface}`);
  }
  if (usability.kind === "infrastructure") {
    explanation.push("它是后台支撑包，通常由对话、任务或其他页面自动调用。");
  }
  if (usability.limitation) {
    explanation.push(`当前限制：${usability.limitation}`);
  }
  if (explanation.length === 0) {
    explanation.push("云雀会读取它的声明信息，但当前没有发现可自动调度的能力。");
  }

  return explanation;
}

export function capabilitySurfaceLabels(manifest: PackManifest): string[] {
  const flags = packFeatureFlags(manifest);
  const labels: string[] = [];
  if (flags.hasFrontend) labels.push(flags.isIframeBundle ? "独立界面包" : "有界面入口");
  if (flags.hasBackend) labels.push("有后端能力");
  if (flags.hasWasm) labels.push("含 WASM");
  if (flags.hasSdk) labels.push("有 SDK");
  if (flags.canRollback) labels.push("可回滚");
  return labels;
}

export function entryInstallRequest(entry: EntryLike): PackInstallRequest | null {
  if (entry.package_url) {
    return {
      packageUrl: entry.package_url,
      sha256: entry.sha256,
      source: entry.source,
      download: true,
    };
  }
  if (entry.manifest_url) {
    return {
      manifestUrl: entry.manifest_url,
      source: entry.source,
      download: entry.downloadable,
    };
  }
  if (entry.manifest_path) {
    return {
      manifestPath: entry.manifest_path,
      source: entry.source,
      download: entry.downloadable,
    };
  }
  return null;
}

export function catalogActionForEntry(entry: EntryLike): PackCatalogAction {
  if (entry.update_action === "update") {
    return { kind: "update", label: "更新", disabled: !Boolean(entryInstallRequest(entry)), needsInstallSource: true };
  }
  if (entry.enabled || entry.update_action === "use") {
    return { kind: "use", label: "已可用", disabled: true, needsInstallSource: false };
  }
  if (entry.installed || entry.update_action === "enable") {
    return { kind: "enable", label: "启用", disabled: false, needsInstallSource: false };
  }
  return { kind: "install", label: "安装", disabled: !Boolean(entryInstallRequest(entry)), needsInstallSource: true };
}

export function formatPackInstallError(error: unknown, fallback = "安装失败"): string {
  const raw = error instanceof Error ? error.message : String(error || "");
  const text = raw.toLowerCase();
  if (/sha|checksum|digest|hash/.test(text)) return "安装失败：SHA256 校验不一致，请确认包来源或重新下载。";
  if (/signature|sign/.test(text)) return "安装失败：签名验证未通过，请确认发布者和签名配置。";
  if (/manifest|schema|invalid json|parse/.test(text)) return "安装失败：manifest 不合法或能力包结构不完整。";
  if (/platform|os|arch|unsupported/.test(text)) return "安装失败：当前平台不支持这个能力包。";
  if (/download|fetch|network|timeout|connection|404|403|500/.test(text)) return "安装失败：下载失败，请检查网络、源地址或权限。";
  return raw ? `${fallback}：${raw}` : fallback;
}
