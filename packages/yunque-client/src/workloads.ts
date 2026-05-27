export interface WorkloadPreset {
  id: string;
  title: string;
  subtitle: string;
  description: string;
  badge?: string;
  capabilities: string[];
}

export const WORKLOAD_PRESETS: WorkloadPreset[] = [
  {
    id: "browser-rpa",
    title: "浏览器 / RPA",
    subtitle: "像 Visual Studio workload 一样把常用能力打包",
    description: "把浏览器意图、回放和扩展场景合成一个用户可选的工作负载。",
    capabilities: ["browser.intent.plan", "rpa.replay.dry_run", "rpa.executor.plan"],
  },
  {
    id: "memory-review",
    title: "记忆 / 回溯",
    subtitle: "适合复盘、验收和长期事实沉淀",
    description: "把快照、差异、回滚计划和审计验证放到一条复盘路径里。",
    capabilities: [
      "memory_time_travel.snapshot_at",
      "memory_time_travel.diff",
      "memory_time_travel.rollback_plan",
      "memory_time_travel.audit.verify",
    ],
  },
  {
    id: "wasm-workload",
    title: "WASM / 插件",
    subtitle: "更像桌面开发的能力包，而不是开发者专属 API",
    description: "把 WASM 插件注册、加载、审批和远程安装当成一个可选工作负载。",
    capabilities: [
      "wasm.host_abi.plan",
      "wasm.remote_install.plan",
      "wasm.remote_install.approval_plan",
      "wasm.plugin.execute",
    ],
  },
  {
    id: "ops-guardrails",
    title: "守护 / 观测",
    subtitle: "给高风险能力加上安全围栏",
    description: "把依赖漂移、守护规则和混沌探针合成一个运维型工作负载。",
    capabilities: [
      "sbom.ci_gate.plan",
      "guardrail_fuzzer.ci_gate.plan",
      "chaos_probe.scheduler.plan",
    ],
  },
  {
    id: "cogni-lab",
    title: "Cogni / 实验",
    subtitle: "查看云雀核心声明层，而不是普通能力包",
    description: "Cogni 是所有工作负载背后的组织原则；这里作为实验入口暴露声明、经验、进化和联邦观测面。",
    badge: "核心解释层",
    capabilities: [
      "cognis.generate",
      "cognis.workflows",
      "cognis.experience",
      "cognis.evolution",
    ],
  },
];

export function listWorkloadPresets(): WorkloadPreset[] {
  return WORKLOAD_PRESETS.map((preset) => ({ ...preset, capabilities: [...preset.capabilities] }));
}

export function getWorkloadPresetById(id: string | null | undefined): WorkloadPreset | undefined {
  if (!id) return undefined;
  const preset = WORKLOAD_PRESETS.find((item) => item.id === id);
  return preset ? { ...preset, capabilities: [...preset.capabilities] } : undefined;
}

export function formatWorkloadCapabilities(preset: Pick<WorkloadPreset, "capabilities">): string {
  return preset.capabilities.join(", ");
}

export function buildWorkloadCatalogHref(preset: Pick<WorkloadPreset, "id">): string {
  return `/packs?preset=${encodeURIComponent(preset.id)}`;
}
