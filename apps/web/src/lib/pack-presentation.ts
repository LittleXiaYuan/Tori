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

export type PackInstallChecklistItem = {
  key: "source" | "permissions" | "boundary" | "rollback";
  label: string;
  detail: string;
  tone: "safe" | "warning" | "danger";
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

export type PackDeliveryProfile = {
  level: "ready" | "support" | "plan_only" | "needs_meat";
  label: string;
  description: string;
  nextStep: string;
  tone: "success" | "primary" | "warning" | "danger";
};

export type PackVerificationStep = {
  key: "trigger" | "observe" | "decide";
  label: string;
  detail: string;
  href?: string;
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

export function packDeliveryProfile(manifest: PackManifest): PackDeliveryProfile {
  const readiness = packReadiness(manifest);
  const usability = packUsability(manifest);
  const flags = packFeatureFlags(manifest);
  const limitation = usability.limitation || manifest.metadata?.limitation || "";
  const planOnlyHint = [
    manifest.id,
    manifest.description,
    limitation,
    ...packExamples(manifest, 5),
    ...(manifest.backend?.capabilities || []),
    ...(manifest.backend?.routeSpecs || []).map((route) => route.description || ""),
  ].join(" ").toLowerCase();

  if (readiness.missing.length > 0) {
    return {
      level: "needs_meat",
      label: "需打磨",
      description: "能力可能已经存在，但用途、入口或后端声明还没讲完整，用户装上后不容易验证价值。",
      nextStep: `交给小羽先补 ${readiness.missing.join("、")}，再预览差异、审计并重新打包。`,
      tone: "danger",
    };
  }

  if (
    usability.kind === "experimental" ||
    flags.isIframeBundle ||
    /plan-only|dry-run|计划|演示|参考包|不执行|不会自动|只生成|门禁|blocked|handoff/.test(planOnlyHint)
  ) {
    return {
      level: "plan_only",
      label: "实验/计划",
      description: "可以体验、验证边界或生成计划，但不应包装成稳定可交付能力。",
      nextStep: "先保留限制说明；如果要变成主路径，下一轮补真实执行、结果查看和回滚证据。",
      tone: "warning",
    };
  }

  if (usability.kind === "infrastructure" || usability.kind === "documented") {
    return {
      level: "support",
      label: "后台支撑",
      description: "它不一定单独打开，而是在 Chat、任务、记忆、知识或设置流程里被云雀调用。",
      nextStep: "从它声明的用户感知位置验证：能否在主路径里看到效果、结果或状态变化。",
      tone: "primary",
    };
  }

  return {
    level: "ready",
    label: "可直接交付",
    description: "有明确入口、示例和能力声明，用户可以直接打开或通过主路径验证结果。",
    nextStep: "安装/启用后打开入口，确认能看到结果、产物、状态或下一步操作。",
    tone: "success",
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

export function packPermissionSummary(manifest: PackManifest): string {
  const groups = groupPackPermissions(manifest.backend?.permissions || []);
  const risk = riskProfileForPack(manifest);
  const permissionText = groups.length > 0
    ? `权限：${groups.map((group) => group.label).join("、")}`
    : "权限：未声明额外权限";
  return `${permissionText}；${risk.requiresAuthorization ? "需要授权后使用" : risk.level === "medium" ? "启用前建议确认" : "低风险"}`;
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

export function packInstallChecklist(
  manifest: PackManifest,
  options: { sourceLabel?: string; installed?: boolean; enabled?: boolean } = {},
): PackInstallChecklistItem[] {
  const groups = groupPackPermissions(manifest.backend?.permissions || []);
  const risk = riskProfileForPack(manifest);
  const flags = packFeatureFlags(manifest);
  const sourceDetail = options.sourceLabel
    ? `来源：${options.sourceLabel}。安装前可先在工坊只读检查包内容、SHA 与能力声明。`
    : options.installed
      ? "来源：本机已安装记录。可从详情页查看版本、入口和权限声明。"
      : "来源：未标注安装源。建议先确认发布者、SHA 和能力声明后再安装。";
  const permissionDetail = groups.length > 0
    ? `会声明 ${groups.map((group) => group.label).join("、")} 等能力；具体动作仍受云雀后端路由、权限和用户确认约束。`
    : "未声明额外权限；启用后主要按能力声明暴露入口或说明，不会默认获得额外写入能力。";
  const boundaryDetail = manifest.id === "yunque.pack.computer-use"
    ? "边界：当前只让 Planner 生成电脑使用计划，不执行本机桌面控制。"
    : risk.requiresAuthorization
      ? "边界：需要授权的高风险能力不会自动越权执行；启用前应确认来源可信，启用后按具体动作授权。"
      : "边界：不会自动泄露 API Key，不会绕过权限声明，也不能调用未声明的后端路由。";
  const rollbackDetail = flags.canRollback
    ? "回滚：声明支持版本回滚；也可以随时禁用能力包。"
    : options.enabled
      ? "回滚：当前可先禁用；此包未声明版本回滚。"
      : "回滚：安装后可禁用；此包未声明版本回滚。";

  return [
    {
      key: "source",
      label: "确认来源",
      detail: sourceDetail,
      tone: options.sourceLabel || options.installed ? "safe" : "warning",
    },
    {
      key: "permissions",
      label: "理解权限",
      detail: permissionDetail,
      tone: groups.some((group) => ["computer", "browser", "sandbox", "admin"].includes(group.key)) ? "warning" : "safe",
    },
    {
      key: "boundary",
      label: "能力边界",
      detail: boundaryDetail,
      tone: risk.requiresAuthorization ? "danger" : "safe",
    },
    {
      key: "rollback",
      label: "回滚路径",
      detail: rollbackDetail,
      tone: flags.canRollback ? "safe" : "warning",
    },
  ];
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

export function packVerificationSteps(manifest: PackManifest): PackVerificationStep[] {
  const usability = packUsability(manifest);
  const delivery = packDeliveryProfile(manifest);
  const metadata = manifest.metadata || {};
  const primaryPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
  const primaryLabel = usability.primaryActionLabel || "打开入口";
  const surface = typeof metadata.usageSurface === "string" && metadata.usageSurface.trim().length > 0
    ? metadata.usageSurface.trim()
    : "";
  const firstExample = packExamples(manifest, 1)[0];
  const triggerDetail = primaryPath
    ? `从「${primaryLabel}」进入；也可以在 Chat 里用一句话描述目标，让云雀按需调用。`
    : usability.kind === "infrastructure"
      ? `它通常不需要单独打开；从 ${surface || "Chat、任务、记忆或知识主路径"} 触发相关流程即可。`
      : `先在能力包详情确认权限和边界，再从 Chat 或任务里尝试触发。`;
  const observeDetail = surface
    ? `到 ${surface} 查看状态、结果、产物或提示是否出现。`
    : primaryPath
      ? `回到 ${primaryPath} 查看页面状态、结果或下一步动作。`
      : "回到 Chat、任务中心或能力包详情查看是否出现状态变化、结果说明或错误提示。";
  const planOnlySuffix = delivery.level === "plan_only"
    ? " 它仍属于实验/计划能力，重点验证计划、报告或预演结果，不要期待自动执行真实动作。"
    : "";
  const decideDetail = delivery.level === "needs_meat"
    ? "如果仍看不出用途，交给小羽补用途、入口、示例、权限边界和回滚说明，再重新打包验证。"
    : usability.limitation
      ? `对照当前限制判断是否符合预期：${usability.limitation} 不够清楚时交给小羽补验证步骤和结果位置。`
      : "结果符合预期就启用或固定入口；不符合预期就禁用、回滚，或交给小羽继续改包。";

  return [
    {
      key: "trigger",
      label: "先触发一次",
      detail: firstExample ? `${triggerDetail} 试法：${firstExample}` : triggerDetail,
      href: primaryPath,
    },
    {
      key: "observe",
      label: "看结果在哪",
      detail: `${observeDetail}${planOnlySuffix}`,
      href: primaryPath,
    },
    {
      key: "decide",
      label: "决定留下还是改",
      detail: decideDetail,
    },
  ];
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
  if (/manifest|schema|invalid json|parse/.test(text)) return "安装失败：能力声明不合法或能力包结构不完整。";
  if (/platform|os|arch|unsupported/.test(text)) return "安装失败：当前平台不支持这个能力包。";
  if (/download|fetch|network|timeout|connection|404|403|500/.test(text)) return "安装失败：下载失败，请检查网络、源地址或权限。";
  return raw ? `${fallback}：${raw}` : fallback;
}
