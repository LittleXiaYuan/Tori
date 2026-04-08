"use client";

import { useEffect, useState } from "react";
import { Card, Button, Spinner, Chip, Tooltip, TextField, Input, Label, Tabs, Switch, Select, ListBox } from "@heroui/react";
import { api, type BrowserStatus, type OPPItem, type BrowserScenario } from "@/lib/api";
import { Globe, RefreshCw, Camera, Monitor, ShieldAlert, Check, XCircle, Play, Zap, Download, ExternalLink, ChevronRight } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { BrowserSessionCard } from "@/components/browser-session-card";
import { useBrowserBridge } from "@/lib/use-browser-bridge";

export default function BrowserPage() {
  const [screenshot, setScreenshot] = useState<string | null>(null);
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
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
  const [extConnected, setExtConnected] = useState(false);

  const {
    bridgeState,
    bridgeActionPending,
    bridgeNotice,
    sendBridgeAction,
  } = useBrowserBridge({
    onActionError: (_action, _payload, message) => {
      showToast(message, "error");
    },
  });

  useEffect(() => {
    api.browserStatus().then(setBrowserStatus).catch(() => {});
    api.browserOPPPending().then((r) => setOppItems(r.items || [])).catch(() => {});
    api.browserConfig().then((c) => setBrowserConfig(c as unknown as Record<string, unknown>)).catch(() => {});
    api.browserScreenshotLatest().then((r) => {
      if (r.screenshot) { setScreenshot(r.screenshot); setScreenshotTs(r.timestamp || ""); }
    }).catch(() => {});
    api.browserExtStatus().then((s) => setExtConnected(s.connected)).catch(() => {});
    api.browserExtScenarios().then((r) => setScenarios(r.scenarios || [])).catch(() => {});
  }, []);

  // Auto-refresh screenshot stream
  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(async () => {
      try {
        const res = await api.browserScreenshotLatest();
        if (res.screenshot) { setScreenshot(res.screenshot); setScreenshotTs(res.timestamp || ""); }
      } catch { /* ignore */ }
    }, refreshInterval * 1000);
    return () => clearInterval(timer);
  }, [autoRefresh, refreshInterval]);

  const navigate = async () => {
    if (!url) return;
    setLoading(true);
    const ts = new Date().toLocaleTimeString();
    setActionLog(prev => [`[${ts}] Navigate →${url}`, ...prev].slice(0, 50));
    try {
      const res = await api.browserNavigate(url);
      if (res.screenshot) { setScreenshot(res.screenshot); setScreenshotTs(ts); }
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [OK] Navigate done`, ...prev].slice(0, 50));
    } catch (e) {
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [FAIL] Navigate failed: ${e}`, ...prev].slice(0, 50));
    }
    setLoading(false);
  };

  const takeScreenshot = async () => {
    setLoading(true);
    try {
      const res = await api.browserScreenshot();
      if (res.screenshot) { setScreenshot(res.screenshot); setScreenshotTs(new Date().toLocaleTimeString()); }
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [OK] Screenshot captured`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "截图失败", "error");
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [FAIL] Screenshot failed`, ...prev].slice(0, 50));
    }
    setLoading(false);
  };

  const runOcr = async () => {
    setLoading(true);
    setActionLog(prev => [`[${new Date().toLocaleTimeString()}] OCR (${ocrMode})...`, ...prev].slice(0, 50));
    try {
      const res = await api.browserOcr(ocrMode);
      setOcrResult(res.text || res.result || "");
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [OK] OCR done (${(res.text || res.result || "").length} chars)`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "OCR 失败", "error");
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [FAIL] OCR failed`, ...prev].slice(0, 50));
    }
    setLoading(false);
  };

  const handleOPPDecide = async (id: string, decision: "allow" | "deny") => {
    setDeciding(id);
    try {
      await api.browserOPPDecide(id, decision);
      setOppItems((prev) => prev.filter((i) => i.id !== id));
    } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
    setDeciding(null);
  };

  const runScenario = async (scenarioId: string) => {
    setRunningScenario(scenarioId);
    const ts = new Date().toLocaleTimeString();
    const scenario = scenarios.find((s) => s.id === scenarioId);
    setActionLog(prev => [`[${ts}] 🚀 运行场景: ${scenario?.name || scenarioId}`, ...prev].slice(0, 50));
    try {
      const res = await api.browserExtRunScenario(scenarioId);
      for (const step of res.results || []) {
        const t = new Date().toLocaleTimeString();
        if (step.ok) {
          setActionLog(prev => [`[${t}] [OK] Step ${step.step}: ${step.action}`, ...prev].slice(0, 50));
        } else {
          setActionLog(prev => [`[${t}] [FAIL] Step ${step.step}: ${step.action} — ${step.error}`, ...prev].slice(0, 50));
        }
      }
      // Refresh screenshot after scenario
      try {
        const ss = await api.browserScreenshot();
        if (ss.screenshot) { setScreenshot(ss.screenshot); setScreenshotTs(new Date().toLocaleTimeString()); }
      } catch { /* ignore */ }
      showToast(`场景 "${scenario?.name}" 执行完成`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "场景执行失败", "error");
      setActionLog(prev => [`[${new Date().toLocaleTimeString()}] [FAIL] 场景执行失败: ${e}`, ...prev].slice(0, 50));
    }
    setRunningScenario(null);
  };

  return (
    <div className="page-root space-y-4 animate-fade-in-up">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Globe size={20} style={{ color: "var(--yunque-accent)" }} />
          <h1 className="page-title">{"浏览器控制"}</h1>
          {browserStatus && (
            <Chip size="sm" style={{
              background: browserStatus.connected ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)",
              color: browserStatus.connected ? "#22c55e" : "#ef4444",
              fontSize: "var(--text-2xs)",
            }}>
              {browserStatus.connected ? "已连接" : "未连接"}
            </Chip>
          )}
          <Chip size="sm" style={{
            background: extConnected ? "rgba(59,130,246,0.1)" : "rgba(156,163,175,0.1)",
            color: extConnected ? "#3b82f6" : "#9ca3af",
            fontSize: "var(--text-2xs)",
          }}>
            {extConnected ? "扩展已连接" : "扩展未连接"}
          </Chip>
        </div>
        <div className="flex items-center gap-2">
          {/* Auto-refresh toggle */}
          <div className="flex items-center gap-1.5 px-2 py-1 rounded-lg" style={{ background: autoRefresh ? "rgba(34,197,94,0.1)" : "transparent" }}>
            <Switch isSelected={autoRefresh} onChange={setAutoRefresh} size="sm" aria-label="自动刷新">
              <Switch.Control><Switch.Thumb /></Switch.Control>
            </Switch>
            <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>自动刷新</span>
            {autoRefresh && (
              <Select selectedKey={String(refreshInterval)} onSelectionChange={(k) => setRefreshInterval(Number(k))} className="w-[56px]" aria-label="刷新间隔">
                <Select.Trigger className="text-[10px] h-5 min-h-0 px-1"><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {[1,2,3,5,10].map(s => <ListBox.Item key={String(s)} id={String(s)} textValue={`${s}s`}>{s}s</ListBox.Item>)}
                  </ListBox>
                </Select.Popover>
              </Select>
            )}
          </div>
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={takeScreenshot} isPending={loading}>
              <Camera size={16} />
            </Button>
            <Tooltip.Content>{"截图"}</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={() => {
              api.browserStatus().then(setBrowserStatus).catch(() => {});
              takeScreenshot();
            }}>
              <RefreshCw size={16} />
            </Button>
            <Tooltip.Content>{"刷新状态"}</Tooltip.Content>
          </Tooltip>
        </div>
      </div>

      {/* Installation guide (when extension not connected) */}
      {!extConnected && (
        <Card className="p-0 overflow-hidden">
          <div className="p-4" style={{ background: "linear-gradient(135deg, rgba(59,130,246,0.08), rgba(139,92,246,0.08))" }}>
            <div className="flex items-start gap-3">
              <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
                style={{ background: "rgba(59,130,246,0.15)" }}>
                <Download size={20} style={{ color: "#3b82f6" }} />
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-sm mb-1">?? Yunque Browser Connector</h3>
                <p className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>
                  ???????????????????? WebSocket ??? Agent API Key??????????????????
                </p>
                <div className="space-y-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                  <div className="flex items-start gap-2">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
                      style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>1</span>
                    <span>?? Chrome / Edge??? <code className="px-1 py-0.5 rounded text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>chrome://extensions</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
                      style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>2</span>
                    <span>??<strong>?????</strong>??? <code className="px-1 py-0.5 rounded text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>yunque-agent/browser-extension</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
                      style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>3</span>
                    <span>???????? <strong>Agent Address</strong> ??? <code className="px-1 py-0.5 rounded text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>ws://localhost:9090/ws/browser</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
                      style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>4</span>
                    <span>? <strong>Agent API Key</strong> ??? <code className="px-1 py-0.5 rounded text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>DEFAULT_API_KEY</code> ????????? Connect</span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
                      style={{ background: "rgba(34,197,94,0.15)", color: "#22c55e" }}>5</span>
                    <span>?????? <strong style={{ color: "#22c55e" }}>ON</strong> ???????????</span>
                  </div>
                </div>
                <p className="text-[10px] mt-3" style={{ color: "var(--yunque-text-muted)" }}>
                  ????? :9090??????? AGENT_ADDR?????????? WebSocket ???
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
        onAction={(type, extra) => sendBridgeAction(type, type === "bridge/takeover" ? { reason: "User takeover from Yunque browser page", ...extra } : extra || {})}
      />

      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="浏览器控制">
            <Tabs.Tab id="browser">{"浏览器"}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="ocr"><Tabs.Separator />OCR<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="opp">
              <Tabs.Separator />
              {"OPP 审批（预览）"} {oppItems.length > 0 && <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444", fontSize: "var(--text-2xs)" }}>{oppItems.length}</Chip>}
              <Tabs.Indicator />
            </Tabs.Tab>
            <Tabs.Tab id="log"><Tabs.Separator />{"操作日志"} {actionLog.length > 0 && <Chip size="sm" style={{ background: "rgba(59,130,246,0.1)", color: "#3b82f6", fontSize: "var(--text-2xs)" }}>{actionLog.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="scenarios"><Tabs.Separator /><Zap size={12} className="inline mr-1" />{"测试场景"} {scenarios.length > 0 && <Chip size="sm" style={{ background: "rgba(139,92,246,0.1)", color: "#8b5cf6", fontSize: "var(--text-2xs)" }}>{scenarios.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="config"><Tabs.Separator />{"配置"}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="browser">
          <Card className="section-card mt-4 overflow-hidden">
            {screenshot ? (
              <div className="p-2">
                <img src={`data:image/png;base64,${screenshot}`} alt="Browser screenshot" className="w-full rounded-lg" style={{ border: "1px solid var(--yunque-border)" }} />
                {screenshotTs && <div className="text-[10px] text-center mt-1" style={{ color: "var(--yunque-text-muted)" }}>截图时间: {screenshotTs}</div>}
                {autoRefresh && <div className="text-[10px] text-center" style={{ color: "#22c55e" }}>●实时刷新中({refreshInterval}s)</div>}
              </div>
            ) : (
              <div className="flex items-center justify-center py-20">
                <div className="text-center">
                  <Monitor size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                  <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"输入 URL 并导航，或点击截图查看当前页面"}</div>
                  {browserStatus?.current_url && (
                    <div className="text-xs mt-2 font-mono" style={{ color: "var(--yunque-text-secondary)" }}>{browserStatus.current_url}</div>
                  )}
                </div>
              </div>
            )}
          </Card>
        </Tabs.Panel>

        <Tabs.Panel id="ocr">
          <div className="mt-4 space-y-4">
            <div className="flex items-center gap-2 flex-wrap">
              {["dom", "tesseract", "vision", "manual"].map((mode) => (
                <button key={mode} onClick={() => setOcrMode(mode)}
                  className="filter-pill filter-pill-subtle" data-active={ocrMode === mode}>
                  {mode.toUpperCase()}
                </button>
              ))}
              <Button size="sm" onPress={runOcr} isPending={loading} className="btn-accent">{"执行 OCR"}</Button>
            </div>
            <Card className="section-card p-4">
              {ocrResult ? (
                <pre className="text-sm font-mono whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>{ocrResult}</pre>
              ) : (
                <div className="text-sm text-center py-8" style={{ color: "var(--yunque-text-muted)" }}>{"选择 OCR 模式并执行"}</div>
              )}
            </Card>
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="opp">
          <div className="mt-4 space-y-3">
            {oppItems.length === 0 ? (
              <Card className="section-card p-12 text-center">
                <ShieldAlert size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"当前版本暂未启用浏览器 OPP 审批队列"}</div>
                <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                  为避免误导用户，这里先按预览态展示；后续接入真实待审批事件后再开放允许 / 拒绝操作。
                </div>
              </Card>
            ) : oppItems.map((item) => (
              <Card key={item.id} className="section-card p-5 hover-lift">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.action}</div>
                    {item.url && <div className="text-xs font-mono mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{item.url}</div>}
                    {item.detail && <div className="text-xs mt-1" style={{ color: "var(--yunque-text-secondary)" }}>{item.detail}</div>}
                    <div className="flex items-center gap-2 mt-2">
                      <Chip size="sm" style={{ background: item.risk_level === "critical" ? "rgba(239,68,68,0.1)" : "rgba(245,158,11,0.1)", color: item.risk_level === "critical" ? "#ef4444" : "#f59e0b", fontSize: "var(--text-2xs)" }}>
                        {item.risk_level}
                      </Chip>
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{new Date(item.created_at).toLocaleString()}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button size="sm" isPending={deciding === item.id} onPress={() => handleOPPDecide(item.id, "allow")} style={{ background: "rgba(34,197,94,0.12)", color: "#22c55e" }}>
                      <Check size={14} /> {"允许"}
                    </Button>
                    <Button size="sm" isPending={deciding === item.id} onPress={() => handleOPPDecide(item.id, "deny")} style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}>
                      <XCircle size={14} /> {"拒绝"}
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="log">
          <div className="mt-4">
            <Card className="section-card p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>操作日志</div>
                {actionLog.length > 0 && (
                  <Button size="sm" variant="ghost" onPress={() => setActionLog([])}>清空</Button>
                )}
              </div>
              {actionLog.length === 0 ? (
                <div className="text-sm text-center py-8" style={{ color: "var(--yunque-text-muted)" }}>暂无操作记录</div>
              ) : (
                <div className="space-y-1 max-h-[60vh] overflow-y-auto">
                  {actionLog.map((log, i) => (
                    <div key={i} className="text-xs font-mono px-2 py-1 rounded" style={{ background: "var(--yunque-bg-hover)", color: log.includes("[FAIL]") ? "#ef4444" : log.includes("[OK]") ? "#22c55e" : "var(--yunque-text-muted)" }}>
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
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>需要安装浏览器扩展</div>
                    <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>请安装 Yunque Browser Connector 扩展后刷新此页面</div>
                  </div>
                </div>
              </Card>
            )}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {scenarios.map((s) => (
                <Card key={s.id} className="section-card p-4 hover-lift cursor-pointer" style={{ transition: "all 0.2s" }}>
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="text-lg">{s.icon}</span>
                      <div>
                        <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{s.name}</div>
                        <div className="text-[11px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{s.steps.length} 步</div>
                      </div>
                    </div>
                    <Button
                      size="sm"
                      isIconOnly
                      isPending={runningScenario === s.id}
                      isDisabled={!extConnected || !!runningScenario}
                      onPress={() => runScenario(s.id)}
                      className="btn-accent"
                    >
                      <Play size={14} />
                    </Button>
                  </div>
                  <div className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{s.description}</div>
                </Card>
              ))}
            </div>
            {scenarios.length === 0 && extConnected && (
              <Card className="section-card p-12 text-center">
                <Zap size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"暂无测试场景"}</div>
              </Card>
            )}
          </div>
        </Tabs.Panel>

        <Tabs.Panel id="config">
          <div className="mt-4">
            <Card className="section-card p-4">
              <div className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>浏览器配置</div>
              {browserStatus && (
                <div className="space-y-2 mb-4">
                  <div className="flex justify-between text-xs"><span style={{ color: "var(--yunque-text-muted)" }}>连接状态</span><span>{browserStatus.connected ? "已连接" : "未连接"}</span></div>
                  {browserStatus.current_url && <div className="flex justify-between text-xs gap-4"><span className="shrink-0" style={{ color: "var(--yunque-text-muted)" }}>当前 URL</span><span className="truncate font-mono">{browserStatus.current_url}</span></div>}
                  {browserStatus.page_title && <div className="flex justify-between text-xs gap-4"><span className="shrink-0" style={{ color: "var(--yunque-text-muted)" }}>页面标题</span><span className="truncate">{browserStatus.page_title}</span></div>}
                </div>
              )}
              {browserConfig ? (
                <pre className="text-xs font-mono p-3 rounded-lg whitespace-pre-wrap overflow-x-auto" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-muted)" }}>
                  {JSON.stringify(browserConfig, null, 2)}
                </pre>
              ) : (
                <div className="text-sm text-center py-4" style={{ color: "var(--yunque-text-muted)" }}>无法获取配置</div>
              )}
            </Card>
          </div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
