"use client";

import { useEffect, useState } from "react";
import { Card, Button, Chip, Tooltip, Tabs, Switch, Select, ListBox } from "@heroui/react";
import { api, type BrowserStatus, type OPPItem, type BrowserScenario } from "@/lib/api";
import { Globe, RefreshCw, Camera, Monitor, ShieldAlert, Check, XCircle, Play, Zap, Download } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { BrowserSessionCard } from "@/components/browser-session-card";
import { useBrowserBridge } from "@/lib/use-browser-bridge";
import { useI18n } from "@/lib/i18n";

export default function BrowserPage() {
  const [screenshot, setScreenshot] = useState<string | null>(null);
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

  const { t } = useI18n();

  const { bridgeState, bridgeActionPending, bridgeNotice, lastArtifact, sendBridgeAction } = useBrowserBridge({
    onActionError: (_action, _payload, message) => {
      showToast(message, "error");
    },
  });

  useEffect(() => {
    api.browserStatus().then(setBrowserStatus).catch(() => {});
    api.browserOPPPending().then((r) => setOppItems(r.items || [])).catch(() => {});
    api.browserConfig().then((c) => setBrowserConfig(c as unknown as Record<string, unknown>)).catch(() => {});
    api.browserScreenshotLatest().then((r) => {
      if (r.screenshot) {
        setScreenshot(r.screenshot);
        setScreenshotTs(r.timestamp || "");
      }
    }).catch(() => {});
    api.browserExtStatus().then((s) => setExtConnected(s.connected)).catch(() => {});
    api.browserExtScenarios().then((r) => setScenarios(r.scenarios || [])).catch(() => {});
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(async () => {
      try {
        const res = await api.browserScreenshotLatest();
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
    setLoading(true);
    try {
      const res = await api.browserScreenshot();
      if (res.screenshot) {
        setScreenshot(res.screenshot);
        setScreenshotTs(new Date().toLocaleTimeString());
      }
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [OK] Screenshot captured`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Action failed", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] Screenshot failed`, ...prev].slice(0, 50));
    }
    setLoading(false);
  };

  const runOcr = async () => {
    setLoading(true);
    setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] OCR (${ocrMode})...`, ...prev].slice(0, 50));
    try {
      const res = await api.browserOcr(ocrMode);
      setOcrResult(res.text || res.result || "");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [OK] OCR done (${(res.text || res.result || "").length} chars)`, ...prev].slice(0, 50));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "OCR failed", "error");
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] OCR failed`, ...prev].slice(0, 50));
    }
    setLoading(false);
  };

  const handleOPPDecide = async (id: string, decision: "allow" | "deny") => {
    setDeciding(id);
    try {
      await api.browserOPPDecide(id, decision);
      setOppItems((prev) => prev.filter((item) => item.id !== id));
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Action failed", "error");
    }
    setDeciding(null);
  };

  const runScenario = async (scenarioId: string) => {
    setRunningScenario(scenarioId);
    const ts = new Date().toLocaleTimeString();
    const scenario = scenarios.find((item) => item.id === scenarioId);
    setActionLog((prev) => [`[${ts}] Run scenario: ${scenario?.name || scenarioId}`, ...prev].slice(0, 50));
    try {
      const res = await api.browserExtRunScenario(scenarioId);
      for (const step of res.results || []) {
        const t = new Date().toLocaleTimeString();
        if (step.ok) {
          setActionLog((prev) => [`[${t}] [OK] Step ${step.step}: ${step.action}`, ...prev].slice(0, 50));
        } else {
          setActionLog((prev) => [`[${t}] [FAIL] Step ${step.step}: ${step.action} - ${step.error}`, ...prev].slice(0, 50));
        }
      }
      try {
        const shot = await api.browserScreenshot();
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
      setActionLog((prev) => [`[${new Date().toLocaleTimeString()}] [FAIL] Scenario failed: ${e}`, ...prev].slice(0, 50));
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
              background: extConnected ? "rgba(59,130,246,0.1)" : "rgba(156,163,175,0.1)",
              color: extConnected ? "#3b82f6" : "#9ca3af",
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
              <Select selectedKey={String(refreshInterval)} onSelectionChange={(k) => setRefreshInterval(Number(k))} className="w-[56px]" aria-label="Auto refresh">
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
            <Button isIconOnly variant="ghost" size="sm" onPress={takeScreenshot} isPending={loading}>
              <Camera size={16} />
            </Button>
            <Tooltip.Content>{t("browserPage.captureScreenshot")}</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={() => {
              api.browserStatus().then(setBrowserStatus).catch(() => {});
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
          <div className="p-4" style={{ background: "linear-gradient(135deg, rgba(59,130,246,0.08), rgba(139,92,246,0.08))" }}>
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style={{ background: "rgba(59,130,246,0.15)" }}>
                <Download size={20} style={{ color: "#3b82f6" }} />
              </div>
              <div className="flex-1">
                <h3 className="mb-1 text-sm font-semibold">{t("browserPage.install.title")}</h3>
                <p className="mb-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  {t("browserPage.install.desc")}
                </p>
                <div className="space-y-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>1</span>
                    <span>{t("browserPage.install.step1")} <code className="rounded px-1 py-0.5 text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>chrome://extensions</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>2</span>
                    <span>{t("browserPage.install.step2a")} <strong>{t("browserPage.install.step2b")}</strong>{t("browserPage.install.step2c")} <strong>{t("browserPage.install.step2d")}</strong>{t("browserPage.install.step2e")} <code className="rounded px-1 py-0.5 text-[11px]" style={{ background: "var(--yunque-bg-muted)" }}>yunque-agent/browser-extension</code></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>3</span>
                    <span>{t("browserPage.install.step3a")} <strong>{t("browserPage.install.step3b")}</strong></span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold" style={{ background: "rgba(59,130,246,0.15)", color: "#3b82f6" }}>4</span>
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

      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="Browser workspace">
            <Tabs.Tab id="browser">{t("browserPage.tab.browser")}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="ocr"><Tabs.Separator />{t("browserPage.tab.ocr")}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="opp"><Tabs.Separator />{t("browserPage.tab.opp")}{oppItems.length > 0 && <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444", fontSize: "var(--text-2xs)" }}>{oppItems.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="log"><Tabs.Separator />{t("browserPage.tab.actionLog")}{actionLog.length > 0 && <Chip size="sm" style={{ background: "rgba(59,130,246,0.1)", color: "#3b82f6", fontSize: "var(--text-2xs)" }}>{actionLog.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="scenarios"><Tabs.Separator /><Zap size={12} className="mr-1 inline" />{t("browserPage.tab.scenarios")}{scenarios.length > 0 && <Chip size="sm" style={{ background: "rgba(139,92,246,0.1)", color: "#8b5cf6", fontSize: "var(--text-2xs)" }}>{scenarios.length}</Chip>}<Tabs.Indicator /></Tabs.Tab>
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
              <Button size="sm" onPress={runOcr} isPending={loading} className="btn-accent">{t("browserPage.runOcr")}</Button>
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
                <Card key={scenario.id} className="section-card cursor-pointer p-4 hover-lift" style={{ transition: "all 0.2s" }}>
                  <div className="mb-2 flex items-start justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-lg">{scenario.icon}</span>
                      <div>
                        <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{scenario.name}</div>
                        <div className="mt-0.5 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{scenario.steps.length} {t("browserPage.steps")}</div>
                      </div>
                    </div>
                    <Button size="sm" isIconOnly isPending={runningScenario === scenario.id} isDisabled={!extConnected || !!runningScenario} onPress={() => runScenario(scenario.id)} className="btn-accent">
                      <Play size={14} />
                    </Button>
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
