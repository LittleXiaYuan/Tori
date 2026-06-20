import {
  Activity,
  BarChart3,
  Brain,
  BriefcaseBusiness,
  Cpu,
  Database,
  Gauge,
  GitBranch,
  GraduationCap,
  Inbox,
  MessageCircle,
  Network,
  PackageCheck,
  Plug,
  Puzzle,
  Rocket,
  Settings,
  ShieldCheck,
  Sparkles,
  Terminal,
  Wrench,
} from "lucide-react";
import type { ComponentType } from "react";

export interface PackSurfaceItem {
  id: string;
  title: string;
  kind: "可操作" | "基础能力" | "治理";
  summary: string;
  actions: string[];
}

export interface PackSurfaceGuide {
  title: string;
  description: string;
  icon: ComponentType<{ size?: number; "aria-hidden"?: boolean }>;
  items: PackSurfaceItem[];
}

export const packSurfaceGuides = {
  memory: {
    title: "这里承接的能力包",
    description: "记忆不是一个孤立页面，它承接长期偏好、情绪线索和任务复盘结果，让 Chat 与任务能带着上下文继续工作。",
    icon: Brain,
    items: [
      {
        id: "yunque.pack.memory",
        title: "记忆",
        kind: "可操作",
        summary: "查看、搜索、添加和整理短期/中期/长期记忆。",
        actions: ["搜索一条记忆", "手动添加重要偏好", "整理压缩旧记忆"],
      },
      {
        id: "yunque.pack.emotion",
        title: "情绪上下文",
        kind: "基础能力",
        summary: "把对话和渠道消息里的情绪信号沉淀到记忆视图。",
        actions: ["查看情感历史", "确认近期互动倾向", "辅助 Chat 调整语气"],
      },
      {
        id: "yunque.pack.reflection",
        title: "反思",
        kind: "基础能力",
        summary: "把任务反馈和经验策略转成后续可召回的记忆线索。",
        actions: ["查看任务沉淀", "配合夜校复盘", "把经验带回下一次任务"],
      },
    ],
  },
  knowledge: {
    title: "这里承接的能力包",
    description: "知识库负责把外部材料变成可检索上下文，供 Chat、任务和 Cogni 使用。",
    icon: Database,
    items: [
      {
        id: "yunque.pack.knowledge",
        title: "知识库",
        kind: "可操作",
        summary: "导入文件、网页、仓库或纯文本，并管理知识源。",
        actions: ["上传文件", "导入 URL/仓库", "编辑或删除知识源"],
      },
      {
        id: "yunque.pack.retrieval",
        title: "检索",
        kind: "基础能力",
        summary: "把知识片段召回到 Chat 与任务上下文里。",
        actions: ["搜索知识片段", "用筛选缩小范围", "让任务引用知识库"],
      },
      {
        id: "yunque.pack.graph",
        title: "图谱上下文",
        kind: "基础能力",
        summary: "把知识和记忆关系变成可复用的上下文结构。",
        actions: ["检查知识覆盖", "辅助 RAG 召回", "为 Cogni 提供背景"],
      },
    ],
  },
  missions: {
    title: "这里承接的能力包",
    description: "任务中心是工作类能力包的共同入口：从 Chat 发起任务，在这里查看进度、自动化、产物和可恢复现场。",
    icon: BriefcaseBusiness,
    items: [
      {
        id: "yunque.pack.missions",
        title: "任务",
        kind: "可操作",
        summary: "查看任务状态、继续运行、暂停、取消或重启任务。",
        actions: ["查看进行中任务", "打开任务执行页", "处理失败任务"],
      },
      {
        id: "yunque.pack.work",
        title: "工作流",
        kind: "可操作",
        summary: "把项目、工作流、任务执行和产物区连成闭环。",
        actions: ["从模板创建任务", "查看产物", "回到 Chat 继续处理"],
      },
      {
        id: "yunque.pack.scheduler",
        title: "调度与自动化",
        kind: "基础能力",
        summary: "支撑定时任务、触发器、队列、恢复现场和子代理分工。",
        actions: ["创建定时任务", "配置触发器", "恢复暂停现场"],
      },
      {
        id: "yunque.pack.cron",
        title: "定时任务",
        kind: "基础能力",
        summary: "让周期性任务按时间表进入任务中心。",
        actions: ["查看定时列表", "手动运行一次", "删除过期规则"],
      },
      {
        id: "yunque.pack.planner-recovery",
        title: "规划恢复",
        kind: "基础能力",
        summary: "保存任务现场，让失败、暂停或等待输入的任务能继续。",
        actions: ["查看可恢复现场", "继续执行任务", "检查失败原因"],
      },
      {
        id: "yunque.pack.session-queue",
        title: "会话队列",
        kind: "基础能力",
        summary: "把待执行任务排队，避免用户不知道请求去了哪里。",
        actions: ["查看待执行", "刷新队列", "清理无效任务"],
      },
      {
        id: "yunque.pack.state",
        title: "任务状态",
        kind: "基础能力",
        summary: "跟踪目标、焦点、资源和当前运行状态。",
        actions: ["查看状态筛选", "检查资源占用", "恢复当前焦点"],
      },
      {
        id: "yunque.pack.subagents",
        title: "子代理分工",
        kind: "基础能力",
        summary: "为复杂任务准备多代理分工和子上下文。",
        actions: ["查看任务拆分", "进入执行详情", "回看子步骤"],
      },
    ],
  },
  skills: {
    title: "这里承接的能力包",
    description: "技能页承接 Skill 生态：本地技能、市场安装、动态技能审批，以及云雀在任务中实际会调用的工具说明。",
    icon: Wrench,
    items: [
      {
        id: "yunque.pack.skills",
        title: "技能",
        kind: "可操作",
        summary: "查看已安装技能、扫描本地技能目录，并观察使用量和成功率。",
        actions: ["扫描 data/skills", "按类别筛选", "查看使用指标"],
      },
      {
        id: "yunque.pack.market",
        title: "技能市场",
        kind: "基础能力",
        summary: "从本地或远程来源发现可安装技能。",
        actions: ["浏览市场", "搜索技能", "安装社区技能"],
      },
      {
        id: "yunque.pack.skillhub",
        title: "远程技能生态",
        kind: "基础能力",
        summary: "连接 ClawHub/ToriHub，并对远程技能做安全策略检查。",
        actions: ["切换来源", "安装 GitHub 技能", "处理动态技能审批"],
      },
    ],
  },
  workers: {
    title: "这里承接的能力包",
    description: "AI IDE 协作页承接外部执行器：云雀负责派发任务，Cursor、Claude Code、Windsurf 等工具负责实际编码和回报结果。",
    icon: Cpu,
    items: [
      {
        id: "yunque.pack.mcp-dispatch",
        title: "MCP 分派",
        kind: "可操作",
        summary: "管理 Worker MCP 配置和外部执行器列表。",
        actions: ["连接 AI IDE", "复制 MCP 配置", "移除离线执行器"],
      },
      {
        id: "yunque.pack.orchestrator",
        title: "编排守护进程",
        kind: "可操作",
        summary: "检测可用 IDE、启动守护进程，并查看活跃会话。",
        actions: ["检测 IDE", "启动/停止守护", "查看分派状态"],
      },
    ],
  },
  cognis: {
    title: "这里承接的能力包",
    description: "Cogni 是给模型组织可选择能力的声明层：它兼容 Skill、MCP 与能力包，把工具、工作流、经验和触发方式压成更省上下文的助手声明。",
    icon: Sparkles,
    items: [
      {
        id: "yunque.pack.cogni-console",
        title: "Cogni 控制台",
        kind: "可操作",
        summary: "用一句话创建 Cogni，并查看声明、健康、轨迹、经验和演化建议。",
        actions: ["新建 Cogni", "启用/停用声明", "查看运行证据"],
      },
      {
        id: "yunque.pack.cogni-kernel",
        title: "Cogni 内核",
        kind: "基础能力",
        summary: "让 Planner 能读取 Cogni 声明、感知能力包状态，并把选择结果写回运行状态。",
        actions: ["检查内核状态", "导入/导出声明", "观察 Pack 门禁"],
      },
      {
        id: "yunque.pack.cognitive-layer",
        title: "认知层",
        kind: "基础能力",
        summary: "把记忆、反思、夜校和 Cogni 组合成可召回、可复用、可审计的模型上下文。",
        actions: ["查看经验模式", "确认演化建议", "关联记忆与任务"],
      },
    ],
  },
  dashboard: {
    title: "这里承接的能力包",
    description: "工作台不是能力包本身，而是云雀启动后的总览入口：看状态、最近任务、成本和运行健康。",
    icon: Gauge,
    items: [
      {
        id: "yunque.pack.workspace",
        title: "工作台",
        kind: "可操作",
        summary: "汇总启动状态、最近任务、提醒和主路径入口。",
        actions: ["开始对话", "进入任务中心", "检查配置提醒"],
      },
      {
        id: "yunque.pack.heartbeat",
        title: "心跳",
        kind: "基础能力",
        summary: "提供自主运行、后台执行和运行日志的状态信号。",
        actions: ["观察运行状态", "排查离线", "回到记忆查看心跳日志"],
      },
      {
        id: "yunque.pack.modules",
        title: "模块诊断",
        kind: "基础能力",
        summary: "展示运行配置档、模块状态和基础诊断信息。",
        actions: ["查看版本", "检查指标", "确认服务在线"],
      },
    ],
  },
  trust: {
    title: "这里承接的能力包",
    description: "信任中心承接高权限能力治理，控制工具、联网、写入、远程包和模型配置的授权边界。",
    icon: ShieldCheck,
    items: [
      {
        id: "yunque.pack.control-plane",
        title: "控制面",
        kind: "治理",
        summary: "查看审批、审计、指标、模型和高风险能力状态。",
        actions: ["处理审批", "查看审计", "检查运行健康"],
      },
      {
        id: "yunque.pack.rbac",
        title: "权限",
        kind: "基础能力",
        summary: "为管理员授权和权限检查提供基础信任分数。",
        actions: ["查看信任分", "授予临时信任", "重置异常权限"],
      },
    ],
  },
  chat: {
    title: "对话里可直接触发的能力包",
    description: "这些能力包没有必要单独占一个页面，它们在 Chat 中通过一句话触发，并把结果落到任务、产物区或会话上下文里。",
    icon: MessageCircle,
    items: [
      {
        id: "yunque.pack.documents",
        title: "文档生成",
        kind: "基础能力",
        summary: "让云雀把报告、表格、页面或演示文稿变成可下载产物。",
        actions: ["说出文档目标", "等待任务产物", "预览或下载文件"],
      },
      {
        id: "yunque.pack.files",
        title: "产物文件",
        kind: "基础能力",
        summary: "列出、预览、下载和继续处理云雀生成的文件。",
        actions: ["查看最近产物", "继续处理文件", "存入知识或记忆"],
      },
      {
        id: "yunque.pack.speech",
        title: "语音",
        kind: "基础能力",
        summary: "支撑语音输入、转写、朗读和可用音色查询。",
        actions: ["录音转文字", "朗读回复", "检查语音能力"],
      },
      {
        id: "yunque.pack.forks",
        title: "会话分支",
        kind: "基础能力",
        summary: "从当前思路分出新路线，同时保留主线可回溯。",
        actions: ["分出新对话", "保留上下文", "回看分支历史"],
      },
      {
        id: "yunque.pack.persona-modes",
        title: "人格模式",
        kind: "基础能力",
        summary: "让 Chat 根据目标切换合适的对话风格和人格上下文。",
        actions: ["检查当前风格", "让云雀建议模式", "回到设置调整 Persona"],
      },
    ],
  },
  innerLife: {
    title: "这里承接的能力包",
    description: "内在生活页把好奇心、梦境和自发反思从后台状态变成可查看、可带回 Chat 的线索。",
    icon: GraduationCap,
    items: [
      {
        id: "yunque.pack.reverie",
        title: "离线梦境",
        kind: "基础能力",
        summary: "记录云雀离线时的自发思考、梦境时间线和可复用线索。",
        actions: ["查看梦境日志", "筛选可行动线索", "带回 Chat 继续处理"],
      },
    ],
  },
  inbox: {
    title: "这里承接的能力包",
    description: "Inbox 是多渠道消息的落点：外部渠道、手动消息和协作提醒都会先进入这里，再交给 Chat 或任务处理。",
    icon: Inbox,
    items: [
      {
        id: "yunque.pack.channels",
        title: "渠道消息",
        kind: "基础能力",
        summary: "把 Chat、IM、外部连接器和手动消息汇入统一收件箱。",
        actions: ["查看未读", "新建消息", "把消息转成任务"],
      },
    ],
  },
  metrics: {
    title: "这里承接的能力包",
    description: "指标页负责把模型、任务和能力调用的成本与健康状态变成可观察信号。",
    icon: BarChart3,
    items: [
      {
        id: "yunque.pack.cost",
        title: "成本与用量",
        kind: "基础能力",
        summary: "跟踪请求、Token、延迟、错误和模型用量。",
        actions: ["查看成功率", "观察 Token 消耗", "排查最近错误"],
      },
    ],
  },
  plugins: {
    title: "这里承接的能力包",
    description: "插件页承接第三方扩展和脚本工具，适合调试本地插件、SDK 桥和可热重载能力。",
    icon: Puzzle,
    items: [
      {
        id: "yunque.pack.plugin-api",
        title: "插件 API",
        kind: "基础能力",
        summary: "为第三方扩展、脚本工具和 SDK 桥提供管理入口。",
        actions: ["新建插件", "编辑插件文件", "刷新并加载变更"],
      },
    ],
  },
  settings: {
    title: "这里承接的能力包",
    description: "设置页承接本机配置、人格、身份、桌面壳和安全访问路径；首次配置仍优先走 /setup。",
    icon: Settings,
    items: [
      {
        id: "yunque.pack.desktop",
        title: "桌面壳",
        kind: "基础能力",
        summary: "连接桌面应用、控制台显示和开机启动等本地体验。",
        actions: ["检查桌面相关配置", "查看运行目录", "回到桌面启动验证"],
      },
      {
        id: "yunque.pack.identity",
        title: "身份",
        kind: "基础能力",
        summary: "合并多渠道身份，支撑用户画像和个性化上下文。",
        actions: ["检查用户配置", "确认渠道身份", "保护本地数据"],
      },
      {
        id: "yunque.pack.instructions",
        title: "指令",
        kind: "基础能力",
        summary: "维护系统指令、记忆上下文和 Chat 行为偏好。",
        actions: ["搜索指令字段", "调整常用设置", "保存后回 Chat 验证"],
      },
      {
        id: "yunque.pack.persona",
        title: "Persona",
        kind: "基础能力",
        summary: "管理人格设定、提示词片段和对话风格。",
        actions: ["打开个性化", "调整人格风格", "在 Chat 中试用"],
      },
      {
        id: "yunque.pack.connectors",
        title: "连接器",
        kind: "基础能力",
        summary: "管理外部服务连接，供技能和任务执行调用。",
        actions: ["进入连接器设置", "测试外部服务", "回到任务使用"],
      },
      {
        id: "yunque.pack.notifications",
        title: "通知",
        kind: "基础能力",
        summary: "控制任务分享、协作提醒和通知渠道。",
        actions: ["进入通知设置", "确认提醒渠道", "处理协作消息"],
      },
    ],
  },
  setup: {
    title: "这里承接的能力包",
    description: "Setup 是新用户第一条主路径：配好模型后，Chat、任务、记忆、知识和能力包才能真正工作。",
    icon: Rocket,
    items: [
      {
        id: "yunque.pack.tori",
        title: "Tori 接入",
        kind: "基础能力",
        summary: "支持 Tori 账号绑定、模型中转、用量查询和 API Key 分支。",
        actions: ["选择 Tori 或 API Key", "测试 Provider", "开始对话"],
      },
    ],
  },
  tools: {
    title: "这里承接的能力包",
    description: "工具执行页用于受控命令、沙箱和跨实例委派；它应该是高权限动作的观察入口，不是普通用户的第一屏。",
    icon: Terminal,
    items: [
      {
        id: "yunque.pack.sandbox",
        title: "沙箱",
        kind: "基础能力",
        summary: "支撑受控命令执行、云桌面沙箱和安全隔离。",
        actions: ["设置工作目录", "执行受控命令", "停止运行会话"],
      },
      {
        id: "yunque.pack.ide",
        title: "IDE 能力",
        kind: "基础能力",
        summary: "为代码审查、IDE 插件和任务编排提供底层能力。",
        actions: ["检查执行输出", "结合 Workers 分派", "回到任务验收"],
      },
      {
        id: "yunque.pack.federation",
        title: "跨实例委派",
        kind: "基础能力",
        summary: "为跨实例能力委派和工具协作准备通道。",
        actions: ["查看工具会话", "确认委派结果", "停止异常任务"],
      },
    ],
  },
  trace: {
    title: "这里承接的能力包",
    description: "执行轨迹页把任务、工具、Cogni 和 Planner 过程变成可回放证据，方便排错与复盘。",
    icon: Activity,
    items: [
      {
        id: "yunque.pack.trace",
        title: "执行轨迹",
        kind: "基础能力",
        summary: "查看最近事件、按任务 ID 定位步骤，或用 trace id 回放一次执行。",
        actions: ["查看最近轨迹", "按任务查询", "回放 trace"],
      },
    ],
  },
  workflows: {
    title: "这里承接的能力包",
    description: "工作流页承接事件驱动自动化：把自然语言目标变成可编辑 DAG，并通过触发器定时或按事件启动。",
    icon: GitBranch,
    items: [
      {
        id: "yunque.pack.triggers",
        title: "触发器",
        kind: "基础能力",
        summary: "把时间、事件和外部信号转成工作流启动条件。",
        actions: ["生成工作流", "编辑 DAG", "查看执行实例"],
      },
    ],
  },
} satisfies Record<string, PackSurfaceGuide>;

export type PackSurfaceKey = keyof typeof packSurfaceGuides;

export function packSurfaceGuide(surface: PackSurfaceKey): PackSurfaceGuide {
  return packSurfaceGuides[surface];
}

export function allGuidedPackIDs(): string[] {
  return Object.values(packSurfaceGuides).flatMap((guide) => guide.items.map((item) => item.id));
}
