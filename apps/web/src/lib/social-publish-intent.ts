export type SocialPublishPlatform = "xiaohongshu" | "x";

export interface SocialPublishIntent {
  platform: SocialPublishPlatform;
  platformName: string;
  scenarioId: string;
  targetUrl: string;
  title: string;
  body: string;
  sourceText: string;
}

export interface SocialPublishStepRecord {
  index: number;
  label: string;
  actionType: string;
  ok: boolean;
  error?: string;
  hasScreenshot?: boolean;
  url?: string;
  title?: string;
}

export interface SocialPublishActionResultLike {
  ok?: boolean;
  error?: string;
  screenshot?: string;
  url?: string;
  currentUrl?: string;
  title?: string;
  tabId?: number | null;
  state?: unknown;
}

const PLATFORM_CONFIG: Record<SocialPublishPlatform, {
  platformName: string;
  scenarioId: string;
  targetUrl: string;
}> = {
  xiaohongshu: {
    platformName: "小红书",
    scenarioId: "xiaohongshu-post-direct",
    targetUrl: "https://creator.xiaohongshu.com/publish/publish",
  },
  x: {
    platformName: "X/Twitter",
    scenarioId: "x-post-direct",
    targetUrl: "https://x.com/home",
  },
};

function detectPlatform(input: string): SocialPublishPlatform | null {
  if (/(小红书|小红薯|xiaohongshu|red\s*note|rednote|redbook)/i.test(input)) {
    return "xiaohongshu";
  }
  if (/(推特|推文|tweet|twitter|x\/twitter|\bx\b)/i.test(input)) {
    return "x";
  }
  return null;
}

function hasPublishVerb(input: string): boolean {
  return /(发帖|发一条|发个|发一个|发布|发表|直发|发笔记|发小红书|发推|推文|发出去|发出|投稿|发行帖子?|post|publish|tweet)/i.test(input);
}

function isDraftOnlyRequest(input: string): boolean {
  const normalized = input.toLowerCase();
  if (/(do not publish|don't publish|only draft|draft only|save as draft)/i.test(normalized)) {
    return true;
  }
  if (/(不要|别|不|暂时不|先不).{0,8}(发布|发出|发帖|直发|发表|post|publish|tweet)/i.test(input)) {
    return true;
  }
  if (/(只|仅|只要|仅需|先|暂时).{0,8}(生成|写|出|做|整理).{0,12}(草稿|文案|内容|脚本|标题)/i.test(input)) {
    return true;
  }
  if (/(保存|放进|存为).{0,4}草稿/i.test(input)) {
    return true;
  }
  return false;
}

function normalizeExtractedText(value: string): string {
  return value
    .replace(/^[\s:："'“”‘’`]+/, "")
    .replace(/[\s"'“”‘’`]+$/, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

function extractQuotedText(input: string): string {
  const match = input.match(/[“"]([^”"]{6,})[”"]|[‘']([^’']{6,})[’']/);
  return normalizeExtractedText(match?.[1] || match?.[2] || "");
}

function extractTitle(input: string): string {
  const match = input.match(/(?:标题|题目)\s*[:：]\s*([^\n。；;，,]{2,40})(?=\s*(?:，|,|。|；|;|\n|正文|内容|文案|$))/i);
  return normalizeExtractedText(match?.[1] || "");
}

function extractBody(input: string): string {
  const labeled = input.match(/(?:正文|内容|文案|笔记内容|帖子内容|推文内容)\s*[:：]\s*([\s\S]+)/i);
  if (labeled?.[1]) return normalizeExtractedText(labeled[1]);

  const colonAfterVerb = input.match(/(?:发帖|发布|发表|直发|发笔记|发推|推文|post|publish|tweet)\s*(?:一条|一个|一篇)?\s*[:：]\s*([\s\S]+)/i);
  if (colonAfterVerb?.[1]) return normalizeExtractedText(colonAfterVerb[1]);

  const quoted = extractQuotedText(input);
  if (quoted) return quoted;

  return "";
}

function deriveTopic(input: string, platform: SocialPublishPlatform): string {
  const platformWords = platform === "xiaohongshu"
    ? /(小红书|小红薯|xiaohongshu|red\s*note|rednote|redbook)/ig
    : /(推特|推文|tweet|twitter|x\/twitter|\bx\b)/ig;
  const cleaned = input
    .replace(platformWords, "")
    .replace(/(帮我|请|麻烦|可以|能否|在|到|把|给|直接|立即|一键|一条|一个|一篇|这个|那个)/g, "")
    .replace(/(发帖|发一条|发个|发一个|发布|发表|直发|发笔记|发小红书|发推|推文|发出去|发出|投稿|发行帖子?|post|publish|tweet)/ig, "")
    .replace(/(标题|题目|正文|内容|文案|笔记|帖子)[:：]?/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return normalizeExtractedText(cleaned).slice(0, 80);
}

function buildDefaultTitle(topic: string): string {
  if (!topic) return "云雀自动化效率演示";
  const normalized = topic.replace(/[。！？!?，,；;：:]+/g, " ").trim();
  if (!normalized) return "云雀自动化效率演示";
  return normalized.length > 24 ? normalized.slice(0, 24) : normalized;
}

function buildDefaultBody(platform: SocialPublishPlatform, topic: string): string {
  if (platform === "x") {
    return topic
      ? `云雀 Agent 效率演示：${topic}。从对话识别意图，到打开页面、填写内容并直接发布，把重复操作交给自动化完成。`
      : "云雀 Agent 正在演示浏览器自动化：从对话识别发帖意图，到打开页面、填写内容并直接发布，全链路减少重复操作。";
  }
  return topic
    ? `今天用云雀 Agent 演示内容运营自动化：${topic}。\n\n我只需要在 Chat 里说出目标，云雀就会规划步骤、打开小红书创作中心、填写标题和正文，并在页面条件满足后直接点击发布。`
    : "今天用云雀 Agent 演示内容运营自动化：自动打开小红书创作中心、填写标题和正文、截图确认，并在满足平台条件后直接点击发布。";
}

export function detectSocialPublishIntent(input: string): SocialPublishIntent | null {
  const sourceText = input.trim();
  if (!sourceText) return null;
  const platform = detectPlatform(sourceText);
  if (!platform || !hasPublishVerb(sourceText) || isDraftOnlyRequest(sourceText)) return null;

  const cfg = PLATFORM_CONFIG[platform];
  const explicitTitle = extractTitle(sourceText);
  const explicitBody = extractBody(sourceText);
  const topic = deriveTopic(sourceText, platform);
  const body = explicitBody || buildDefaultBody(platform, topic);
  const title = platform === "xiaohongshu"
    ? (explicitTitle || buildDefaultTitle(topic || body))
    : "";

  return {
    platform,
    platformName: cfg.platformName,
    scenarioId: cfg.scenarioId,
    targetUrl: cfg.targetUrl,
    title,
    body,
    sourceText,
  };
}

export function buildSocialPublishActions(intent: SocialPublishIntent): Record<string, unknown>[] {
  if (intent.platform === "x") {
    return [
      { type: "browser_navigate", url: intent.targetUrl },
      { type: "browser_screenshot" },
      {
        type: "browser_click",
        target: { strategy: "bySelector", selector: `[data-testid="tweetTextarea_0"]` },
      },
      {
        type: "browser_input",
        target: { strategy: "bySelector", selector: `[data-testid="tweetTextarea_0"]` },
        text: intent.body,
      },
      { type: "browser_screenshot" },
      {
        type: "browser_click",
        target: { strategy: "bySelector", selector: `[data-testid="tweetButtonInline"], [data-testid="tweetButton"]` },
      },
      { type: "browser_screenshot" },
    ];
  }

  return [
    { type: "browser_navigate", url: intent.targetUrl },
    { type: "browser_screenshot" },
    {
      type: "browser_input",
      target: { strategy: "bySelector", selector: `input[placeholder*="标题"], textarea[placeholder*="标题"]` },
      text: intent.title,
    },
    {
      type: "browser_input",
      target: { strategy: "bySelector", selector: `[contenteditable="true"], textarea[placeholder*="正文"], .ql-editor, .ProseMirror` },
      text: intent.body,
    },
    { type: "browser_screenshot" },
    { type: "browser_click", target: { strategy: "byText", text: "发布" } },
    { type: "browser_screenshot" },
  ];
}

export function socialPublishStepLabel(action: Record<string, unknown>, intent: SocialPublishIntent): string {
  const type = String(action.type || "");
  if (type === "browser_navigate") return `打开${intent.platformName}发布入口`;
  if (type === "browser_screenshot") return "截图留痕";
  if (type === "browser_click") {
    const target = action.target as Record<string, unknown> | undefined;
    if (target?.strategy === "byText") return `点击“${target.text || "发布"}”`;
    return "点击发布编辑区";
  }
  if (type === "browser_input") {
    const target = action.target as Record<string, unknown> | undefined;
    const selector = String(target?.selector || "");
    if (selector.includes("标题")) return "填写标题";
    return "填写正文";
  }
  return type || "浏览器动作";
}

export function stepRecordFromResult(
  index: number,
  label: string,
  action: Record<string, unknown>,
  result: SocialPublishActionResultLike | null | undefined,
): SocialPublishStepRecord {
  return {
    index,
    label,
    actionType: String(action.type || ""),
    ok: Boolean(result?.ok),
    error: result?.error,
    hasScreenshot: Boolean(result?.screenshot),
    url: result?.url || result?.currentUrl,
    title: result?.title,
  };
}

export function formatSocialPublishConnectorRequired(intent: SocialPublishIntent): string {
  return [
    `我理解你的意思：这不是“先去 Pack 页面点按钮”，而是应该在 Chat 里直接触发 **${intent.platformName}直发**。`,
    "",
    "现在可以继续，但需要先连接 **Yunque Browser Connector**，因为真正点击发布必须发生在你的已登录浏览器会话里。",
    "",
    "- 我会从这条对话识别发布意图",
    `- 打开 ${intent.platformName} 发布入口`,
    intent.title ? `- 填写标题：${intent.title}` : "- 填写正文",
    "- 点击发布，并把截图/失败步骤回传到右侧电脑面板",
    "",
    "连接入口：[/packs/browser](/packs/browser)。连接后点上方“继续浏览器任务”即可从这条消息恢复。",
  ].join("\n");
}

export function formatSocialPublishResult(
  intent: SocialPublishIntent,
  steps: SocialPublishStepRecord[],
  failure?: string,
): string {
  const completed = steps.filter((step) => step.ok).length;
  const lines: string[] = [];
  if (failure) {
    lines.push(`**${intent.platformName}直发已启动，但在执行中被平台页面中断。**`);
    lines.push("");
    lines.push(`卡住位置：${steps.find((step) => !step.ok)?.label || "浏览器动作"}`);
    lines.push(`原因：${failure}`);
  } else {
    lines.push(`**${intent.platformName}直发流程已从对话执行完成。**`);
    lines.push("");
    lines.push("我已经按这条消息完成：识别意图 → 打开发布入口 → 填写内容 → 点击发布 → 截图留痕。");
  }
  lines.push("");
  lines.push(`- 平台：${intent.platformName}`);
  if (intent.title) lines.push(`- 标题：${intent.title}`);
  lines.push(`- 正文：${intent.body.length > 80 ? `${intent.body.slice(0, 80)}...` : intent.body}`);
  lines.push(`- 已执行步骤：${completed}/${steps.length}`);
  if (steps.some((step) => step.hasScreenshot)) lines.push("- 截图：已写入浏览器面板最近动作");
  lines.push("");
  lines.push("真实直发仍取决于账号登录、素材要求、验证码/风控弹窗和平台 DOM 变化；如果这些条件不满足，我会回传卡住步骤，而不是只生成草稿。");
  return lines.join("\n");
}
