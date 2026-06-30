"use client";

import { useEffect, useState } from "react";
import { Card, Button, Chip, Tooltip, Tabs, Switch, Select, ListBox } from "@heroui/react";
import { createBrowserIntentPackClient, type BrowserActPlan, type BrowserStatus, type OPPItem, type BrowserScenario } from "@/lib/browser-intent-pack-client";
import { Globe, RefreshCw, Camera, Monitor, ShieldAlert, Check, XCircle, Play, Zap, Download, Cloud, CloudOff, Loader2, ExternalLink, Search, ArrowUpDown, Keyboard, Link2, Hand, FileText } from "lucide-react";

const BRAND_ICONS: Record<string, string> = {
  google: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.27-4.74 3.27-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>',
  baidu: '<svg viewBox="0 0 24 24" fill="#2319DC"><path d="M5.927 12.497c2.063-.443 1.782-2.909 1.72-3.352-.101-.652-.753-2.474-2.59-2.376-2.18.116-2.244 2.96-2.244 2.96-.116 1.14.975 3.2 3.114 2.768zm3.005-4.41c1.32 0 2.395-1.523 2.395-3.404C11.327 2.804 10.253 1 8.932 1 7.612 1 6.54 2.804 6.54 4.683c0 1.88 1.072 3.404 2.392 3.404zm5.553.427c1.636.238 2.894-1.084 3.118-2.865.232-1.785-.832-3.32-2.467-3.551-1.636-.237-2.9 1.085-3.126 2.866-.228 1.78.84 3.318 2.475 3.55zm4.903 3.42s-.116-2.844-2.244-2.96c-1.837-.1-2.49 1.724-2.59 2.376-.062.443-.343 2.909 1.72 3.351 2.14.433 3.23-1.627 3.114-2.768zm-4.082 2.126c-.458-.457-1.62-1.597-1.62-1.597-.655-.653-1.42-.442-1.42-.442-.655.12-1.063.552-1.063.552-.34.322-1.305 1.308-1.305 1.308-1.187 1.076-.675 2.152-.675 2.152.323 1.108 1.38 1.333 1.38 1.333 1.213.332 2.338-.107 2.338-.107 1.475-.44 1.6-1.216 1.6-1.216.443-.55.765-1.983.765-1.983z"/></svg>',
  youtube: '<svg viewBox="0 0 24 24" fill="#FF0000"><path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>',
  duckduckgo: '<svg viewBox="0 0 24 24" fill="#DE5833"><path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm0 2.182a9.818 9.818 0 1 1 0 19.636 9.818 9.818 0 0 1 0-19.636z"/><circle cx="9.5" cy="9" r="2" fill="#fff"/><circle cx="9.5" cy="9" r="1" fill="#2D4F8E"/></svg>',
  linkedin: '<svg viewBox="0 0 24 24" fill="#0A66C2"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 0 1-2.063-2.065 2.064 2.064 0 1 1 2.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>',
  x: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>',
  gmail: '<svg viewBox="0 0 24 24"><path d="M24 5.457v13.909c0 .904-.732 1.636-1.636 1.636h-3.819V11.73L12 16.64l-6.545-4.91v9.273H1.636A1.636 1.636 0 0 1 0 19.366V5.457c0-2.023 2.309-3.178 3.927-1.964L5.455 4.64 12 9.548l6.545-4.91 1.528-1.145C21.69 2.28 24 3.434 24 5.457z" fill="#EA4335"/></svg>',
  bing: '<svg viewBox="0 0 24 24" fill="#008373"><path d="M5.71 0v18.013l4.55 2.558 8.028-4.658v-4.46L10.26 8.056V2.832L5.71 0zm4.55 10.76l8.028 3.397v4.46l-8.028 4.383V10.76z"/></svg>',
  wikipedia: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12.09 13.119c-.936 1.932-2.217 4.548-2.853 5.728-.616 1.074-1.127.931-1.532.029-1.406-3.321-4.293-9.144-5.651-12.409-.251-.601-.441-.987-.619-1.139-.181-.15-.554-.24-1.122-.271C.103 5.033 0 4.982 0 4.898v-.455l.052-.045c.924-.005 5.401 0 5.401 0l.051.045v.434c0 .119-.075.176-.225.176l-.564.031c-.485.029-.727.164-.727.436 0 .135.053.33.166.601 1.082 2.646 4.818 10.521 4.818 10.521l2.681-5.467-2.5-5.054c-.203-.47-.377-.683-.554-.747-.203-.074-.597-.132-1.143-.172-.123-.013-.166-.065-.166-.173v-.453l.052-.044s3.575-.023 3.575-.023l.052.044v.402c0 .157-.058.237-.196.237-.571.015-.855.125-.855.341 0 .12.08.335.218.667l1.843 3.667 1.854-3.667c.148-.335.218-.556.218-.674 0-.182-.252-.314-.749-.341-.136 0-.218-.071-.218-.237v-.402l.052-.044s2.843.023 2.843.023l.051.044v.453c0 .108-.053.16-.166.173-.564.04-.94.098-1.143.172-.177.064-.351.277-.554.747l-2.5 5.054 2.681 5.467s3.736-7.875 4.818-10.521c.113-.271.166-.466.166-.601 0-.272-.242-.407-.727-.436l-.564-.031c-.15 0-.225-.057-.225-.176v-.434l.051-.045s4.477.005 5.401 0l.052.045v.455c0 .084-.103.135-.335.159-.568.031-.941.121-1.122.271-.178.152-.368.538-.619 1.139-1.358 3.265-4.245 9.088-5.651 12.409-.405.902-.916 1.045-1.532-.029-.636-1.18-1.917-3.796-2.853-5.728z"/></svg>',
  github: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>',
  hackernews: '<svg viewBox="0 0 24 24" fill="#F0652F"><path d="M0 0v24h24V0H0zm11.52 14.34L8.1 7.68h1.86l2.22 4.56 2.16-4.56h1.86l-3.42 6.66V18h-1.26v-3.66z"/></svg>',
};

function ScenarioIcon({ icon }: { icon: string }) {
  const svg = BRAND_ICONS[icon];
  if (svg) return <span className="inline-block w-5 h-5" dangerouslySetInnerHTML={{ __html: svg }} />;
  const fallback: Record<string, React.ReactNode> = {
    scroll: <ArrowUpDown size={18} />,
    keyboard: <Keyboard size={18} />,
    link: <Link2 size={18} />,
    hand: <Hand size={18} />,
  };
  return <>{fallback[icon] || <Search size={18} />}</>;
}
import { showToast } from "@/components/toast-provider";
import { confirmAction } from "@/components/confirm-dialog";
import { BrowserSessionCard } from "@/components/browser-session-card";
import { useBrowserBridge } from "@/lib/use-browser-bridge";
import { openExternal } from "@/lib/safe-url";
import { useI18n } from "@/lib/i18n";
import { formatErrorMessage } from "@/lib/error-utils";

const browserIntentClient = createBrowserIntentPackClient();
const directPublishScenarioIds = new Set(["xiaohongshu-post-direct", "x-post-direct"]);

const capabilityCards = [
  {
    icon: <Monitor size={16} />,
    title: "看见网页现场",
    desc: "连接浏览器扩展后，可查看当前页面、截图留痕，并把页面内容交给云雀理解。",
  },
  {
    icon: <Search size={16} />,
    title: "提取页面信息",
    desc: "支持 DOM / OCR / 视觉等读取方式，用于整理页面文字、表格和关键状态。",
  },
  {
    icon: <Play size={16} />,
    title: "运行审核过的场景",
    desc: "可执行预置浏览器场景；涉及发布、填写、点击等高风险动作时保留人工确认入口。",
  },
  {
    icon: <FileText size={16} />,
    title: "生成动作计划",
    desc: "把“打开、点击、输入、截图、提取”转成可审阅计划；当前不会直接控制本机桌面。",
  },
];

const planStatusCards = [
  { key: "browser_act_plan_ready", label: "动作计划", ready: "已生成", pending: "待生成" },
  { key: "permission_gate_ready", label: "权限检查", ready: "已检查", pending: "待检查" },
  { key: "runtime_skill_gate_ready", label: "运行能力", ready: "已匹配", pending: "待匹配" },
  { key: "opp_gate_ready", label: "人工审批", ready: "已接入", pending: "待接入" },
  { key: "browser_act_ready", label: "真实执行", ready: "可执行", pending: "仅计划" },
  { key: "network_access", label: "联网动作", ready: "会联网", pending: "不联网" },
] as const;

export default function BrowserIntentPackPage() {
  const [screenshot, setScreenshot] = useState<string | null>(null);
  const [screenshotLoading, setScreenshotLoading] = useState(false);
  const [ocrLoading, setOcrLoading] = useState(false);
  const [ocrMode, setOcrMode] = useState("dom");
  const [ocrResult, setOcrResult] = useState<string>("");
  const [tab, setTab] = useState("browser");
  const [browserStatus, setBrowserStatus] = useState<BrowserStatus | null>(null);
  const [oppItems, setOppItems] = useState<OPPItem[]>([]);
  const [deciding, setDeciding] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(3);
  const [browserConfig, setBrowserConfig] = useState<Record<string, unknown> | null>(null);
  const [actionLog, setActionLog] = useState<string[]>([]);
  const [screenshotTs, setScreenshotTs] = useState<string>("");
  const [scenarios, setScenarios] = useState<BrowserScenario[]>([]);
  const [runningScenario, setRunningScenario] = useState<string | null>(null);
  const [publishMode, setPublishMode] = useState<"direct" | "review">("direct");
  const [extConnected, setExtConnected] = useState(false);
  const [desktopSandbox, setDesktopSandbox] = useState<{ id: string; stream_url: string; created_at: string; vnc_log?: string[] } | null>(null);
  const [desktopLoading, setDesktopLoading] = useState(false);
  const [browserActPlan, setBrowserActPlan] = useState<BrowserActPlan | null>(null);
  const [browserActPlanning, setBrowserActPlanning] = useState(false);

  const { t } = useI18n();

  const { bridgeState, bridgeActionPending, bridgeNotice, lastArtifact, sendBridgeAction } = useBrowserBridge({
    onActionError: (_action, _payload, message) => {
      showToast(message, "error");
    },
  });

  useEffect(() => {
    browserIntentClient.status().then(setBrowserStatus).catch(() => {});
    browserIntentClient.oppPending().then((r) => setOppItems(r.items || [])).catch(() => {});
    browserIntentClient.config().then((c) => setBrowserConfig(c as unknown as Record<string, unknown>)).catch(() => {});
    browserIntentClient.screenshotLatest().then((r) => {
      if (r.screenshot) {
        setScreenshot(r.screenshot);
        setScreenshotTs(r.timestamp || "");
      }
    }).catch(() => {});
    browserIntentClient.extensionStatus().then((s) => setExtConnected(s.connected)).catch(() => {});
    browserIntentClient.scenarios().then((r) => setScenarios(r.scenarios || [])).catch(() => {});
    browserIntentClient.desktopStatus().then((r) => { if (r.running && r.sandbox) setDesktopSandbox(r.sandbox); }).catch(() => {});
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(async () => {
      try {
        const res = await browserIntentClient.screenshotLatest();
        if (res.screenshot) {
          setScreenshot(res.screenshot);
          setScreenshotTs(res.timestamp || "");
        }
      } catch {
        // ignore
      }
    }, refreshInterval * 1000);
    return () => clearInterval(timer);
  }, [autoRefresh, refreshInterval]);

  const takeScreenshot = async () => {
    setScreenshotLoading(true);
    try {
      const res = await browserIntentClient.screenshot();
      if (res.screenshot) {
        setScreenshot(res.screenshot);
        setScreenshotTs(new Date().toLocaleTimeString());
      }
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [OK] Screenshot captured`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "截图失败", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] Screenshot failed`, ...prev].slice(0, 50));
    }
    setScreenshotLoading(false);
  };

  const runOcr = async () => {
    setOcrLoading(true);
    setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] OCR (${ocrMode})...`, ...prev].slice(0, 50));
    try {
      const res = await browserIntentClient.ocr(ocrMode);
      setOcrResult(res.text || res.result || "");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [OK] OCR done (${(res.text || res.result || "").length} chars)`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "OCR 失败", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] OCR failed`, ...prev].slice(0, 50));
    }
    setOcrLoading(false);
  };

  const buildBrowserActPlan = async () => {
    setBrowserActPlanning(true);
    try {
      const res = await browserIntentClient.browserActPlan({
        intent: "open_url",
        target_url: typeof browserStatus?.current_url === "string" ? browserStatus.current_url : "https://example.com",
        selector: "button[data-action=export]",
        text: "Export",
        requested_by: "pack-console",
        reason: "operator semantic browser_act review",
        dry_run: true,
      });
      setBrowserActPlan(res.plan);
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [OK] browser_act plan gate ready`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "browser_act 计划生成失败", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] browser_act plan failed`, ...prev].slice(0, 50));
    }
    setBrowserActPlanning(false);
  };

  const handleOPPDecide = async (id: string, decision: "allow" | "deny") => {
    setDeciding(id);
    try {
      await browserIntentClient.oppDecide(id, decision);
      setOppItems((prev) => prev.filter((item) => item.id !== id));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Action failed", "error");
    }
    setDeciding(null);
  };

  const runScenario = async (scenarioId: string) => {
    const scenario = scenarios.find((item) => item.id === scenarioId);
    if (publishMode === "review" && directPublishScenarioIds.has(scenarioId)) {
      showToast("已切换为审核模式：请在工作流页生成可编辑流程，删除/禁用发布节点后再运行。", "warning");
      return;
    }
    if (publishMode === "direct" && directPublishScenarioIds.has(scenarioId)) {
      const confirmed = await confirmAction({
        title: "确认直发内容？",
        body: `将运行「${scenario?.name || scenarioId}」，可能在已登录的平台账号中填写并发布内容。请确认素材、账号和平台合规提示都已检查。`,
        confirmLabel: "确认直发",
        cancelLabel: "取消",
        tone: "danger",
      });
      if (!confirmed) return;
    }
    setRunningScenario(scenarioId);
    const ts = new Date().toLocaleTimeString();
    setActionLog((prev) => [`[${ts}] Run scenario: ${scenario?.name || scenarioId}`, ...prev].slice(0, 50));
    try {
      const res = await browserIntentClient.runScenario(scenarioId);
      for (const step of res.results || []) {
        const t = new Date().toLocaleTimeString();
        if (step.ok) {
          setActionLog((prev) => [`[${t}] [OK] Step ${step.step}: ${step.action}`, ...prev].slice(0, 50));
        } else {
          setActionLog((prev) => [`[${t}] [FAIL] Step ${step.step}: ${step.action} - ${formatErrorMessage(step.error, "步骤未完成")}`, ...prev].slice(0, 50));
        }
      }
      try {
        const shot = await browserIntentClient.screenshot();
        if (shot.screenshot) {
          setScreenshot(shot.screenshot);
          setScreenshotTs(new Date().toLocaleTimeString());
        }
      } catch {
        // ignore
      }
      showToast(`${t("browserPage.scenarioComplete")}: ${scenario?.name || scenarioId}`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Scenario failed", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] Scenario failed: ${formatErrorMessage(e, "场景未完成")}`, ...prev].slice(0, 50));
    }
    setRunningScenario(null);
  };

  return (
    <div className="page-root space-y-4 animate-fade-in-up">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Globe size={20} style={{ color: "var(--yunque-accent)" }} />
          <h1 className="page-title">{t("browserPage.title")}</h1>
          {browserStatus && (
            <Chip
              size="sm"
              style={{
                background: browserStatus.connected ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)",
                color: browserStatus.connected ? "#22c55e" : "#ef4444",
                fontSize: "var(--text-2xs)",
              }}
            >
              {browserStatus.connected ? t("browserPage.connected") : t("browserPage.disconnected")}
            </Chip>
          )}
          <Chip
            size="sm"
            style={{
              background: extConnected ? "var(--yunque-accent-muted)" : "rgba(156,163,175,0.1)",
              color: extConnected ? "var(--yunque-accent-strong)" : "#9ca3af",
              fontSize: "var(--text-2xs)",
            }}
          >
            {extConnected ? t("browserPage.extConnected") : t("browserPage.extDisconnected")}
          </Chip>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 rounded-lg px-2 py-1" style={{ background: autoRefresh ? "rgba(34,197,94,0.1)" : "transparent" }}>
            <Switch isSelected={autoRefresh} onChange={setAutoRefresh} size="sm" aria-label="Auto refresh">
              <Switch.Control><Switch.Thumb /></Switch.Control>
            </Switch>
            <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.autoRefresh")}</span>
            {autoRefresh && (
              <Select selectedKey={String(refreshInterval)} onSelectionChange={(k) => setRefreshInterval(Number(k))} className="w-[56px]" aria-label="自动刷新间隔">
                <Select.Trigger className="h-5 min-h-0 px-1 text-[10px]"><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {[1, 2, 3, 5, 10].map((s) => <ListBox.Item key={String(s)} id={String(s)} textValue={`${s}s`}>{s}s</ListBox.Item>)}
                  </ListBox>
                </Select.Popover>
              </Select>
            )}
          </div>
          <Tooltip delay={0}>
            <Button isIconOnly aria-label={t("browserPage.captureScreenshot")} variant="ghost" size="sm" onPress={takeScreenshot} isPending={screenshotLoading}>
              <Camera size={16} />
            </Button>
            <Tooltip.Content>{t("browserPage.captureScreenshot")}</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button isIconOnly aria-label={t("browserPage.refreshStatus")} variant="ghost" size="sm" onPress={() => {
              browserIntentClient.status().then(setBrowserStatus).catch(() => {});
              takeScreenshot();
            }}>
              <RefreshCw size={16} />
            </Button>
            <Tooltip.Content>{t("browserPage.refreshStatus")}</Tooltip.Content>
          </Tooltip>
        </div>
      </div>

      {!extConnected && (
        <Card className="overflow-hidden p-0">
          <div className="p-4" style={{ background: "linear-gradient(135deg, var(--yunque-accent-soft), rgba(139,92,246,0.08))" }}>
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style={{ background: "var(--yunque-accent-muted)" }}>
                <Download size={20} style={{ color: "var(--yunque-accent-strong)" }} />
              </div>
              <div className="flex-1">
                <h3 className="mb-1 text-sm font-semibold">{t("browserPage.install.title")}</h3>
                <p className="mb-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  {t("browserPage.install.desc")}
                </p>
                <div className="space-y-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)" }}>1</span>
                    <span>{t("browserPage.install.step1")} <code className="rounded px-1 py-0.5 text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>chrome://extensions</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)" }}>2</span>
                    <span>{t("browserPage.install.step2a")} <strong>{t("browserPage.install.step2b")}</strong>{t("browserPage.install.step2c")} <strong>{t("browserPage.install.step2d")}</strong>{t("browserPage.install.step2e")} <code className="rounded px-1 py-0.5 text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>yunque-agent/apps/browser-extension</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)" }}>3</span>
                    <span>{t("browserPage.install.step3a")} <strong>{t("browserPage.install.step3b")}</strong></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)" }}>4</span>
                    <span>{t("browserPage.install.step4a")} <strong>{t("browserPage.install.step4b")}</strong> <code className="rounded px-1 py-0.5 text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>ws://localhost:9090/ws/browser</code>{t("browserPage.install.step4c")}</span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "rgba(34,197,94,0.15)", color: "#22c55e" }}>5</span>
                    <span>{t("browserPage.install.step5a")} <strong style={{ color: "#22c55e" }}>ON</strong>.</span>
                  </div>
                </div>
                <p className="mt-3 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                  {t("browserPage.install.note")}
                </p>
              </div>
            </div>
          </div>
        </Card>
      )}

      <BrowserSessionCard
        state={bridgeState}
        pendingAction={bridgeActionPending}
        notice={bridgeNotice}
        artifact={lastArtifact}
        onAction={(type, extra) => sendBridgeAction(type, type === "bridge/takeover" ? { reason: "User takeover from Yunque browser page", ...extra } : extra || {})}
      />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className="p-5">
            <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
                  <Globe size={18} style={{ color: "var(--yunque-accent)" }} />
                  浏览器能力包能做什么
                </div>
                <p className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                  它把浏览器变成云雀可观察、可审阅、可留痕的工作现场：先看见页面和生成计划，再由你决定是否运行场景或审批高风险动作。
                </p>
              </div>
              <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "#f59e0b" }}>
                当前不执行本机桌面控制
              </Chip>
            </div>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {capabilityCards.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                    <span className="flex h-7 w-7 items-center justify-center rounded-md" style={{ background: "var(--yunque-accent-soft)", color: "var(--yunque-accent-strong)" }}>{item.icon}</span>
                    {item.title}
                  </div>
                  <div className="text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.06)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <ShieldAlert size={16} style={{ color: "#f59e0b" }} />
              当前边界
            </div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              <div>可以：截图、读取页面、运行预置场景、生成可审阅动作计划。</div>
              <div>需要确认：发布、提交、删除、付款、登录态相关动作。</div>
              <div>不会：绕过扩展授权、偷取 token、直接控制本机鼠标键盘或写本地文件。</div>
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <span className="flex h-9 w-9 items-center justify-center rounded-xl" style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}>📕</span>
                  <div>
                    <h2 className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>内容平台直发自动化</h2>
                    <p className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>面向小红书 / X 等运营场景：打开平台、填写内容、截图留痕、点击发布，减少重复操作。</p>
                  </div>
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {["打开创作页", "填写标题/正文", "发布前截图", "点击发布", "发布后留痕"].map((item, idx) => (
                    <Chip key={item} size="sm" style={{ background: idx === 3 ? "rgba(239,68,68,0.10)" : "var(--yunque-accent-soft)", color: idx === 3 ? "#ef4444" : "var(--yunque-accent-strong)", fontSize: "var(--text-2xs)" }}>
                      {idx + 1}. {item}
                    </Chip>
                  ))}
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button size="sm" variant="outline" onPress={() => setTab("scenarios")}>
                  查看场景
                </Button>
                <Button size="sm" className="btn-accent" isDisabled={!extConnected || !!runningScenario} isPending={runningScenario === "xiaohongshu-post-direct"} onPress={() => runScenario("xiaohongshu-post-direct")}>
                  <Play size={14} className="mr-1" /> 小红书直发
                </Button>
              </div>
            </div>
          </div>
          <div className="p-5" style={{ background: "linear-gradient(180deg, rgba(239,68,68,0.08), rgba(245,158,11,0.04))", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>发布策略</div>
                <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>演示默认走完整直发链路。</div>
              </div>
              <Switch isSelected={publishMode === "direct"} onChange={(v) => setPublishMode(v ? "direct" : "review")} size="sm" aria-label="Direct publish mode">
                <Switch.Control><Switch.Thumb /></Switch.Control>
              </Switch>
            </div>
            <div className="rounded-lg p-3 text-xs leading-5" style={{ background: "rgba(0,0,0,0.12)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text-secondary)" }}>
              当前：<strong style={{ color: publishMode === "direct" ? "#ef4444" : "#f59e0b" }}>{publishMode === "direct" ? "直发模式" : "审核模式"}</strong>
              <br />直发要求浏览器扩展已连接、目标账号已登录，并且页面满足平台发布条件（例如素材、验证码、合规提示）。
            </div>
          </div>
        </div>
      </Card>

      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="Browser workspace">
            <Tabs.Tab id="browser">{t("browserPage.tab.browser")}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="ocr"><Tabs.Separator />{t("browserPage.tab.ocr")}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="opp"><Tabs.Separator />{t("browserPage.tab.opp")}{oppItems.length > 0 && <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444", fontSize: "var(--text-2xs)" }}>{oppItems.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="intent"><Tabs.Separator /><FileText size={12} className="mr-1 inline" />动作计划<Chip size="sm" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b", fontSize: "var(--text-2xs)" }}>PLAN</Chip><Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="log"><Tabs.Separator />{t("browserPage.tab.actionLog")}{actionLog.length > 0 && <Chip size="sm" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)", fontSize: "var(--text-2xs)" }}>{actionLog.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="scenarios"><Tabs.Separator /><Zap size={12} className="mr-1 inline" />{t("browserPage.tab.scenarios")}{scenarios.length > 0 && <Chip size="sm" style={{ background: "rgba(139,92,246,0.1)", color: "#8b5cf6", fontSize: "var(--text-2xs)" }}>{scenarios.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="desktop"><Tabs.Separator /><Cloud size={12} className="mr-1 inline" />云电脑{desktopSandbox && <Chip size="sm" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e", fontSize: "var(--text-2xs)" }}>LIVE</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="config"><Tabs.Separator />{t("browserPage.tab.config")}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="browser">
          <Card className="section-card mt-4 overflow-hidden">
            {screenshot ? (
              <div className="p-2">
                <img src={`data:image/png;base64,${screenshot}`} alt="Browser screenshot" className="w-full rounded-lg" style={{ border: "1px solid var(--yunque-border)" }} />
                {screenshotTs && <div className="mt-1 text-center text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.capturedAt")}: {screenshotTs}</div>}
                {autoRefresh && <div className="text-center text-[10px]" style={{ color: "#22c55e" }}>{t("browserPage.liveRefresh")} ({refreshInterval}s)</div>}
              </div>
            ) : (
              <div className="flex items-center justify-center py-20">
                <div className="text-center">
                  <Monitor size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                  <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.empty")}</div>
                  {browserStatus?.current_url && <div className="mt-2 text-xs font-mono" style={{ color: "var(--yunque-text-secondary)" }}>{browserStatus.current_url}</div>}
                </div>
              </div>
            )}
          </Card>
        </Tabs.Panel>

        <Tabs.Panel id="ocr">
          <div className="mt-4 space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              {["dom", "tesseract", "vision", "manual"].map((mode) => (
                <button key={mode} onClick={() => setOcrMode(mode)} className="filter-pill filter-pill-subtle" data-active={ocrMode === mode}>
                  {mode.toUpperCase()}
                </button>
              ))}
              <Button size="sm" onPress={runOcr} isPending={ocrLoading} className="btn-accent">{t("browserPage.runOcr")}</Button>
            </div>
            <Card className="section-card p-4">
              {ocrResult ? (
                <pre className="whitespace-pre-wrap text-sm font-mono" style={{ color: "var(--yunque-text)" }}>{ocrResult}</pre>
              ) : (
                <div className="py-8 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.ocrEmpty")}</div>
              )}
            </Card>
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="opp">
          <div className="mt-4 space-y-3">
            {oppItems.length === 0 ? (
              <Card className="section-card p-12 text-center">
                <ShieldAlert size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.oppEmpty")}</div>
                <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                  {t("browserPage.oppPreview")}
                </div>
              </Card>
            ) : oppItems.map((item) => (
              <Card key={item.id} className="section-card p-5 hover-lift">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.action}</div>
                    {item.url && <div className="mt-0.5 text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>{item.url}</div>}
                    {item.detail && <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{item.detail}</div>}
                    <div className="mt-2 flex items-center gap-2">
                      <Chip size="sm" style={{ background: item.risk_level === "critical" ? "rgba(239,68,68,0.1)" : "rgba(245,158,11,0.1)", color: item.risk_level === "critical" ? "#ef4444" : "#f59e0b", fontSize: "var(--text-2xs)" }}>
                        {item.risk_level}
                      </Chip>
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(item.created_at).toLocaleString()}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button size="sm" isPending={deciding === item.id} onPress={() => handleOPPDecide(item.id, "allow")} style={{ background: "rgba(34,197,94,0.12)", color: "#22c55e" }}>
                      <Check size={14} /> {t("browserPage.allow")}
                    </Button>
                    <Button size="sm" isPending={deciding === item.id} onPress={() => handleOPPDecide(item.id, "deny")} style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}>
                      <XCircle size={14} /> {t("browserPage.deny")}
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="intent">
          <div className="mt-4 space-y-4">
            <Card className="section-card p-5">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    <FileText size={16} /> 动作计划预览
                  </div>
                  <p className="mt-2 max-w-3xl text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                    把「打开页面 / 点击 / 输入 / 截图 / 提取」等目标变成可审阅的步骤。当前是 plan-only：只生成计划和安全检查，不消费浏览器会话、不执行扩展动作、不写浏览器状态、不写文件。
                  </p>
                </div>
                <Button size="sm" onPress={buildBrowserActPlan} isPending={browserActPlanning} className="btn-accent">
                  生成计划
                </Button>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-2 md:grid-cols-6">
                {planStatusCards.map((item) => {
                  const value = browserActPlan?.[item.key];
                  return (
                    <div key={item.key} className="rounded-lg p-2" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                      <div className="truncate text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{item.label}</div>
                      <div className="mt-1 text-xs font-semibold" style={{ color: value ? "#22c55e" : "#f59e0b" }}>{value ? item.ready : item.pending}</div>
                    </div>
                  );
                })}
              </div>
              <div className="mt-3 rounded-lg p-3 text-xs leading-5" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.22)", color: "var(--yunque-text-secondary)" }}>
                这一步用于让用户先审阅“云雀准备怎么做”。只有未来接入执行器、权限、审批和运行时能力都满足时，才会进入真实浏览器动作。
              </div>
            </Card>

            {browserActPlan ? (
              <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
                <Card className="section-card p-4 lg:col-span-2">
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>计划步骤</div>
                  <div className="space-y-2">
                    {browserActPlan.planned_actions.map((action) => (
                      <div key={action.index} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)" }}>
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-xs font-semibold">{action.index + 1}. {action.intent}</span>
                          <Chip size="sm" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)", fontSize: "var(--text-2xs)" }}>{action.executor_action}</Chip>
                        </div>
                        {action.target_url && <div className="mt-2 truncate text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>{action.target_url}</div>}
                        {action.selector && <div className="mt-1 truncate text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>{action.selector}</div>}
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          <Chip size="sm" variant="soft">权限：{action.requires_permission}</Chip>
                          <Chip size="sm" variant="soft">运行能力：{action.requires_runtime_skill}</Chip>
                          {action.requires_opp_gate && <Chip size="sm" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b" }}>需要审批</Chip>}
                          {!action.executes_browser_action && <Chip size="sm" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>当前不执行</Chip>}
                          {action.writes_browser_state && <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444" }}>会改浏览器状态</Chip>}
                        </div>
                      </div>
                    ))}
                  </div>
                </Card>
                <Card className="section-card p-4">
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>为什么还不能自动执行</div>
                  <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                    {(browserActPlan.blocked_by.length > 0 ? browserActPlan.blocked_by : ["执行器、权限和审批全部满足后才会开放真实动作。"]).map((blocker) => (
                      <div key={blocker} className="rounded-md px-3 py-2" style={{ background: "rgba(245,158,11,0.08)" }}>{blocker}</div>
                    ))}
                  </div>
                </Card>
                {[
                  ["权限检查详情", browserActPlan.permission_gate],
                  ["运行能力详情", browserActPlan.runtime_skill_gate],
                  ["审批详情", browserActPlan.opp_gate],
                ].map(([title, gate]) => (
                  <details key={String(title)} className="section-card rounded-xl p-4 lg:col-span-1" style={{ border: "1px solid var(--yunque-border)" }}>
                    <summary className="cursor-pointer text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{String(title)}</summary>
                    <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg p-3 text-[11px] font-mono" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-muted)" }}>
                      {JSON.stringify(gate, null, 2)}
                    </pre>
                  </details>
                ))}
                <Card className="section-card p-4 lg:col-span-3">
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>产物与阻塞原因</div>
                  <div className="flex flex-wrap gap-2">
                    {browserActPlan.artifacts.map((artifact) => (
                      <Chip key={artifact} size="sm" style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)", fontSize: "var(--text-2xs)" }}>{artifact}</Chip>
                    ))}
                    {browserActPlan.blocked_by.map((blocker) => (
                      <Chip key={blocker} size="sm" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b", fontSize: "var(--text-2xs)" }}>{blocker}</Chip>
                    ))}
                  </div>
                </Card>
              </div>
            ) : (
              <Card className="section-card p-12 text-center">
                <FileText size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>尚未生成动作计划。</div>
                <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>点击「生成计划」查看云雀准备怎么做，以及为什么当前不会直接执行。</div>
              </Card>
            )}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="log">
          <div className="mt-4">
            <Card className="section-card p-4">
              <div className="mb-3 flex items-center justify-between">
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{t("browserPage.actionLog")}</div>
                {actionLog.length > 0 && <Button size="sm" variant="ghost" onPress={() => setActionLog([])}>{t("browserPage.clear")}</Button>}
              </div>
              {actionLog.length === 0 ? (
                <div className="py-8 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.noActivity")}</div>
              ) : (
                <div className="max-h-[60vh] space-y-1 overflow-y-auto">
                  {actionLog.map((log, i) => (
                    <div key={i} className="rounded px-2 py-1 text-xs font-mono" style={{ background: "var(--yunque-bg-hover)", color: log.includes("[FAIL]") ? "#ef4444" : log.includes("[OK]") ? "#22c55e" : "var(--yunque-text-muted)" }}>
                      {log}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="scenarios">
          <div className="mt-4 space-y-3">
            {!extConnected && (
              <Card className="section-card p-5">
                <div className="flex items-center gap-3">
                  <Zap size={20} style={{ color: "#f59e0b" }} />
                  <div>
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{t("browserPage.extRequired")}</div>
                    <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.extRequiredDesc")}</div>
                  </div>
                </div>
              </Card>
            )}
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
              {scenarios.map((scenario) => (
                <Card key={scenario.id} className="section-card cursor-pointer p-4 hover-lift" style={{ transition: "all 0.2s", borderColor: directPublishScenarioIds.has(scenario.id) ? "rgba(239,68,68,0.35)" : undefined }}>
                  <div className="mb-2 flex items-start justify-between">
                    <div className="flex items-center gap-2">
                      <ScenarioIcon icon={scenario.icon} />
                      <div>
                        <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                          {scenario.name}
                          {directPublishScenarioIds.has(scenario.id) && <Chip size="sm" style={{ background: "rgba(239,68,68,0.10)", color: "#ef4444", fontSize: "var(--text-2xs)" }}>直发</Chip>}
                        </div>
                        <div className="mt-0.5 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{scenario.steps.length} {t("browserPage.steps")}</div>
                      </div>
                    </div>
                    <Tooltip delay={0}>
                      <Button size="sm" isIconOnly aria-label={`运行场景 ${scenario.name}`} isPending={runningScenario === scenario.id} isDisabled={!extConnected || !!runningScenario || (publishMode === "review" && directPublishScenarioIds.has(scenario.id))} onPress={() => runScenario(scenario.id)} className="btn-accent">
                        <Play size={14} />
                      </Button>
                      <Tooltip.Content>运行</Tooltip.Content>
                    </Tooltip>
                  </div>
                  <div className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{scenario.description}</div>
                </Card>
              ))}
            </div>
            {scenarios.length === 0 && extConnected && (
              <Card className="section-card p-12 text-center">
                <Zap size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.noScenarios")}</div>
              </Card>
            )}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="desktop">
          <div className="mt-4 space-y-4">
            <div className="flex items-center gap-3">
              {desktopSandbox ? (
                <>
                  <Chip size="sm" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
                    <Cloud size={12} className="mr-1 inline" /> {desktopSandbox.id}
                  </Chip>
                  <Button size="sm" variant="outline" onPress={async () => {
                    const confirmed = await confirmAction({
                      title: "停止云电脑？",
                      body: `将销毁当前云电脑 ${desktopSandbox.id}，未保存的浏览器现场和临时状态可能丢失。`,
                      confirmLabel: "停止云电脑",
                      cancelLabel: "取消",
                      tone: "danger",
                    });
                    if (!confirmed) return;
                    setDesktopLoading(true);
                    try {
                      await browserIntentClient.desktopDestroy();
                      setDesktopSandbox(null);
                      showToast("云电脑已停止", "success");
                    } catch (e) { showToast(e instanceof Error ? e.message : "Failed", "error"); }
                    setDesktopLoading(false);
                  }} isPending={desktopLoading} style={{ color: "#ef4444", borderColor: "rgba(239,68,68,0.3)" }}>
                    <CloudOff size={14} className="mr-1" /> 停止
                  </Button>
                  <Button size="sm" variant="ghost" aria-label="刷新云电脑状态" onPress={async () => {
                    try {
                      const r = await browserIntentClient.desktopStatus();
                      if (r.running && r.sandbox) setDesktopSandbox(r.sandbox);
                      else setDesktopSandbox(null);
                    } catch { /* ignore */ }
                  }}>
                    <RefreshCw size={14} />
                  </Button>
                </>
              ) : (
                <Button size="sm" onPress={async () => {
                  setDesktopLoading(true);
                  try {
                    const r = await browserIntentClient.desktopCreate();
                    if (r.ok && r.sandbox) {
                      setDesktopSandbox(r.sandbox);
                      showToast("云电脑已创建", "success");
                    } else {
                      showToast(r.message || "创建云电脑失败", "error");
                    }
                  } catch (e) { showToast(e instanceof Error ? e.message : "Failed", "error"); }
                  setDesktopLoading(false);
                }} isPending={desktopLoading} className="btn-accent">
                  <Cloud size={14} className="mr-1" /> 创建云电脑
                </Button>
              )}
            </div>

            {desktopSandbox?.stream_url ? (
              <Card className="section-card p-6">
                <div className="flex items-center gap-3 mb-4">
                  <div className="flex-1">
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>云电脑运行中</div>
                    <div className="mt-1 text-xs font-mono" style={{ color: "var(--yunque-text-secondary)" }}>ID: {desktopSandbox.id}</div>
                  </div>
                </div>
                <div
                  className="flex flex-col items-center justify-center rounded-lg py-16"
                  style={{ background: "linear-gradient(135deg, var(--yunque-accent-soft), rgba(139,92,246,0.06))", border: "1px dashed var(--yunque-border)" }}
                >
                  <Monitor size={48} className="mb-4" style={{ color: "var(--yunque-accent)" }} />
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>XFCE 桌面已就绪</div>
                  <div className="mb-6 text-xs" style={{ color: "var(--yunque-text-muted)" }}>点击下方按钮在新标签页中打开云电脑</div>
                  <Button size="lg" onPress={() => openExternal(desktopSandbox.stream_url)} className="btn-accent">
                    <ExternalLink size={16} className="mr-2" /> 打开云电脑
                  </Button>
                </div>
                {desktopSandbox.vnc_log && desktopSandbox.vnc_log.length > 0 && (
                  <details className="mt-3">
                    <summary className="cursor-pointer text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>VNC Startup Log</summary>
                    <div className="mt-1 p-3 rounded-lg text-xs font-mono" style={{ background: "rgba(0,0,0,0.3)", color: "var(--yunque-text-secondary)" }}>
                      {desktopSandbox.vnc_log.map((line, i) => <div key={i}>{line}</div>)}
                    </div>
                  </details>
                )}
              </Card>
            ) : desktopSandbox ? (
              <Card className="section-card p-6">
                <Loader2 size={32} className="mx-auto mb-3 animate-spin" style={{ color: "var(--yunque-accent)" }} />
                <div className="text-sm text-center" style={{ color: "var(--yunque-text-muted)" }}>云电脑运行中，正在等待桌面入口...</div>
                <div className="mt-2 text-xs font-mono text-center" style={{ color: "var(--yunque-text-secondary)" }}>ID: {desktopSandbox.id}</div>
                {desktopSandbox.vnc_log && desktopSandbox.vnc_log.length > 0 && (
                  <div className="mt-4 p-3 rounded-lg text-xs font-mono" style={{ background: "rgba(0,0,0,0.3)", color: "var(--yunque-text-secondary)" }}>
                    <div className="mb-1 font-semibold" style={{ color: "var(--yunque-text)" }}>VNC Startup Log:</div>
                    {desktopSandbox.vnc_log.map((line, i) => <div key={i}>{line}</div>)}
                  </div>
                )}
              </Card>
            ) : (
              <Card className="section-card p-12 text-center">
                <Cloud size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>创建一台云电脑，让 Agent 获得完整浏览器和远程工作环境</div>
                <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>需要先在设置中配置云电脑服务密钥</div>
              </Card>
            )}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="config">
          <div className="mt-4">
            <Card className="section-card p-4">
              <div className="mb-3 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{t("browserPage.browserConfig")}</div>
              {browserStatus && (
                <div className="mb-4 space-y-2">
                  <div className="flex justify-between text-xs"><span style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.connection")}</span><span>{browserStatus.connected ? t("browserPage.connected") : t("browserPage.disconnected")}</span></div>
                  {browserStatus.current_url && <div className="flex justify-between gap-4 text-xs"><span className="shrink-0" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.currentUrl")}</span><span className="truncate font-mono">{browserStatus.current_url}</span></div>}
                  {browserStatus.page_title && <div className="flex justify-between gap-4 text-xs"><span className="shrink-0" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.pageTitle")}</span><span className="truncate">{browserStatus.page_title}</span></div>}
                </div>
              )}
              {browserConfig ? (
                <pre className="overflow-x-auto whitespace-pre-wrap rounded-lg p-3 text-xs font-mono" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-muted)" }}>
                  {JSON.stringify(browserConfig, null, 2)}
                </pre>
              ) : (
                <div className="py-4 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("browserPage.noActivity")}</div>
              )}
            </Card>
          </div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
