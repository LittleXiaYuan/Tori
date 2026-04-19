"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, Chip, Input, Label, ProgressBar, Spinner, TextField } from "@heroui/react";
import { api, getAuthHeaders } from "@/lib/api";
import type { SetupEnvironment, SetupTemplate } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Bot,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Cloud,
  Code2,
  Cpu,
  Eye,
  EyeOff,
  ExternalLink,
  Globe,
  Key,
  Layers,
  Monitor,
  RefreshCw,
  Rocket,
  Server,
  Settings,
  XCircle,
  Zap,
} from "lucide-react";

const categoryLabels: Record<string, string> = {
  personal: "个人",
  team: "团队",
  enterprise: "企业",
};

const templateIcons: Record<string, React.ElementType> = {
  "personal-assistant": Bot,
  "team-chat-bot": Layers,
  "code-review-agent": Code2,
  "public-api-service": Globe,
};

// Step indices — kept as named constants to make the flow diagram obvious.
// Tori branch short-circuits from STEP_CHOOSE → STEP_DONE via polling.
const STEP_CHOOSE = 0;
const STEP_DETECT = 1;
const STEP_MODEL = 2;
const STEP_TEMPLATE = 3;
const STEP_DONE = 4;

// Default Tori endpoint — kept here (rather than hard-coded inline) so that
// future self-hosted Tori instances can be surfaced from a config prop without
// touching the render path.
const DEFAULT_TORI_URL = "https://tori.owo.today";

// Poll cadence for /v1/tori/status during OAuth. 1.5s strikes a balance between
// feeling responsive on bind-success and not hammering the server while the
// user is still interacting with the OAuth page.
const TORI_POLL_INTERVAL_MS = 1500;

export default function SetupPage() {
  const router = useRouter();
  const { t } = useI18n();
  const [step, setStep] = useState(STEP_CHOOSE);
  const [checking, setChecking] = useState(true);
  const [env, setEnv] = useState<SetupEnvironment | null>(null);
  const [templates, setTemplates] = useState<SetupTemplate[]>([]);
  const [selectedTpl, setSelectedTpl] = useState("");
  const [detecting, setDetecting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [applying, setApplying] = useState(false);
  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [applyMessage, setApplyMessage] = useState("");
  const [installingId, setInstallingId] = useState<string | null>(null);
  const [installProgress, setInstallProgress] = useState<{ percent: number; detail: string } | null>(null);
  const [installMessage, setInstallMessage] = useState<Record<string, { ok: boolean; msg: string }>>({});
  const [toriBinding, setToriBinding] = useState(false);
  const [toriAuthURL, setToriAuthURL] = useState<string | null>(null);
  const [toriError, setToriError] = useState<string | null>(null);

  const steps = useMemo(
    () => [
      { title: t("setup.step.choose"), icon: Rocket },
      { title: t("setup.step.detect"), icon: Monitor },
      { title: t("setup.step.model"), icon: Zap },
      { title: t("setup.step.template"), icon: Layers },
      { title: t("setup.step.done"), icon: CheckCircle2 },
    ],
    [t]
  );

  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const token = localStorage.getItem("yunque_token");
        const res = await fetch("/v1/auth/status", {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });
        const data = await res.json();
        if (!mounted) return;
        if (data?.password_set && !data?.authenticated) {
          router.replace("/login");
          return;
        }
      } catch {
        // ignore
      } finally {
        if (mounted) setChecking(false);
      }
    })();

    return () => {
      mounted = false;
    };
  }, [router]);

  const runDetect = async () => {
    setDetecting(true);
    try {
      const data = await api.setupDetect();
      setEnv(data);
      const provider = data.providers?.[0];
      if (provider?.base_url) setBaseURL(provider.base_url);
      if (provider?.model) setModel(provider.model);
    } catch {
      setEnv(null);
    } finally {
      setDetecting(false);
    }
  };

  useEffect(() => {
    if (!checking && step === STEP_DETECT && !env) {
      void runDetect();
    }
  }, [checking, step, env]);

  useEffect(() => {
    if (step !== STEP_TEMPLATE || templates.length > 0) return;
    api.setupTemplates()
      .then((data) => {
        const list = data.templates || [];
        setTemplates(list);
        if (!selectedTpl && list[0]) setSelectedTpl(list[0].id);
      })
      .catch(() => undefined);
  }, [step, templates.length, selectedTpl]);

  useEffect(() => {
    if (step !== STEP_DONE) return;
    // Tori branch already persists config server-side via ApplyLLMConfig;
    // skip re-running setupApply so we don't overwrite Tori's LLM settings
    // with empty locals.
    if (!selectedTpl) {
      setApplyMessage(t("setup.done.subtitle"));
      return;
    }
    setApplying(true);
    api.setupApply(selectedTpl, { api_key: apiKey, base_url: baseURL, model })
      .then((data) => setApplyMessage(data.message || t("setup.done.subtitle")))
      .catch((error) => setApplyMessage(error instanceof Error ? error.message : "Setup failed"))
      .finally(() => setApplying(false));
  }, [step, selectedTpl, apiKey, baseURL, model, t]);

  // Poll /v1/tori/status while a bind flow is pending. We stop the polling
  // loop on either (a) success — advance to done, or (b) user stepping away
  // from the choose screen (the unmount cleanup clears the interval).
  useEffect(() => {
    if (!toriBinding) return;
    let cancelled = false;
    const tick = async () => {
      try {
        const status = await api.toriStatus();
        if (cancelled) return;
        if (status.bound) {
          setToriBinding(false);
          setToriAuthURL(null);
          setToriError(null);
          setApplyMessage(t("setup.tori.bound"));
          setStep(STEP_DONE);
        }
      } catch {
        // Transient errors during OAuth are expected; keep polling until the
        // user explicitly cancels.
      }
    };
    const handle = window.setInterval(tick, TORI_POLL_INTERVAL_MS);
    void tick();
    return () => {
      cancelled = true;
      window.clearInterval(handle);
    };
  }, [toriBinding, t]);

  const startToriBind = async () => {
    setToriError(null);
    setToriAuthURL(null);
    setToriBinding(true);
    try {
      const res = await api.toriBind(DEFAULT_TORI_URL);
      // Backend returns either auth_url (new field) or authorize_url (legacy);
      // accept both so a mismatched backend build doesn't strand the user.
      const authURL =
        (res as { auth_url?: string; authorize_url?: string }).auth_url ||
        (res as { auth_url?: string; authorize_url?: string }).authorize_url ||
        null;
      setToriAuthURL(authURL);
    } catch (error) {
      setToriBinding(false);
      setToriError(error instanceof Error ? error.message : t("setup.tori.error"));
    }
  };

  const cancelToriBind = () => {
    setToriBinding(false);
    setToriAuthURL(null);
    setToriError(null);
  };

  const testLLM = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const result = await api.setupTestProvider({ base_url: baseURL, api_key: apiKey, model });
      if (result.ok && result.provider.available) {
        setTestResult({
          ok: true,
          msg: result.provider.latency ? `Connection OK · ${result.provider.latency}` : "Connection OK",
        });
      } else {
        setTestResult({ ok: false, msg: result.provider.error || "Connection failed" });
      }
    } catch (error) {
      setTestResult({ ok: false, msg: error instanceof Error ? error.message : "Connection failed" });
    } finally {
      setTesting(false);
    }
  };

  const installComponent = async (id: string) => {
    setInstallingId(id);
    setInstallProgress({ percent: 0, detail: "Preparing..." });
    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_API_BASE || ""}/v1/setup/install-component`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "text/event-stream", ...getAuthHeaders() },
        body: JSON.stringify({ component_id: id }),
      });

      if (res.headers.get("content-type")?.includes("text/event-stream") && res.body) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const line of lines) {
            if (!line.startsWith("data: ")) continue;
            const payload = JSON.parse(line.slice(6));
            if (payload.stage === "error") {
              setInstallMessage((prev) => ({ ...prev, [id]: { ok: false, msg: payload.detail || "Install failed" } }));
              return;
            }
            if (payload.stage === "done") {
              setInstallMessage((prev) => ({ ...prev, [id]: { ok: true, msg: "Installed" } }));
              await runDetect();
              return;
            }
            setInstallProgress({ percent: payload.percent || 0, detail: payload.detail || "Installing..." });
          }
        }
        return;
      }

      const data = await res.json();
      setInstallMessage((prev) => ({ ...prev, [id]: { ok: !!data.success, msg: data.message || data.error || "Done" } }));
      if (data.success) await runDetect();
    } catch (error) {
      setInstallMessage((prev) => ({ ...prev, [id]: { ok: false, msg: error instanceof Error ? error.message : "Install failed" } }));
    } finally {
      setInstallingId(null);
      setInstallProgress(null);
    }
  };

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-6">
      <div className="text-center">
        <h1 className="text-3xl font-bold">{t("setup.title")}</h1>
        <p className="mt-2 text-sm text-default-500">{t("setup.subtitle")}</p>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-2">
        {steps.map((item, index) => {
          const Icon = item.icon;
          const active = index === step;
          const done = index < step;
          return (
            <div key={item.title} className="flex items-center gap-2">
              <div
                className={`flex items-center gap-2 rounded-full px-4 py-2 text-sm ${
                  active ? "bg-primary text-primary-foreground" : done ? "bg-success/20 text-success" : "bg-default-100 text-default-500"
                }`}
              >
                <Icon size={14} />
                {item.title}
              </div>
              {index < steps.length - 1 && <ChevronRight size={14} className="text-default-400" />}
            </div>
          );
        })}
      </div>

      {step === STEP_CHOOSE && (
        <Card className="p-6">
          <div className="mb-5 text-center">
            <h2 className="text-lg font-semibold">{t("setup.choose.title")}</h2>
            <p className="mt-1 text-sm text-default-500">{t("setup.choose.subtitle")}</p>
          </div>

          {!toriBinding && (
            <div className="grid gap-4 md:grid-cols-2">
              <ChoiceCard
                icon={Cloud}
                accent="#8b5cf6"
                title={t("setup.choose.tori.title")}
                tag={t("setup.choose.tori.tag")}
                desc={t("setup.choose.tori.desc")}
                cta={t("setup.choose.tori.cta")}
                onPress={startToriBind}
              />
              <ChoiceCard
                icon={Key}
                accent="#3b82f6"
                title={t("setup.choose.apikey.title")}
                tag={t("setup.choose.apikey.tag")}
                desc={t("setup.choose.apikey.desc")}
                cta={t("setup.choose.apikey.cta")}
                onPress={() => setStep(STEP_DETECT)}
              />
            </div>
          )}

          {toriBinding && (
            <div className="flex flex-col items-center gap-4 rounded-2xl border border-white/10 bg-white/3 px-4 py-10 text-center">
              <Spinner size="lg" />
              <div className="text-sm font-medium">{t("setup.tori.binding")}</div>
              <div className="text-xs text-default-500">{t("setup.tori.bindingHint")}</div>
              <div className="flex flex-wrap items-center justify-center gap-2">
                {toriAuthURL && (
                  <Button size="sm" variant="ghost" onPress={() => window.open(toriAuthURL, "_blank", "noopener,noreferrer")}>
                    <ExternalLink size={14} /> {t("setup.tori.openAuth")}
                  </Button>
                )}
                <Button size="sm" variant="ghost" onPress={cancelToriBind}>
                  {t("setup.tori.cancel")}
                </Button>
              </div>
            </div>
          )}

          {toriError && (
            <div className="mt-4 rounded-xl border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
              {toriError}
              <Button size="sm" variant="ghost" className="ml-3" onPress={startToriBind}>
                <RefreshCw size={14} /> {t("setup.tori.retry")}
              </Button>
            </div>
          )}

          <div className="mt-6 flex justify-center">
            <button
              type="button"
              className="text-xs text-default-500 underline-offset-4 transition hover:text-default-700 hover:underline"
              onClick={() => router.push("/chat")}
            >
              {t("setup.choose.later")}
            </button>
          </div>
        </Card>
      )}

      {step === STEP_DETECT && (
        <Card className="p-6">
          <div className="mb-5 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <Monitor size={18} /> {t("setup.step.detect")}
            </h2>
            <Button variant="ghost" size="sm" isDisabled={detecting} onPress={runDetect}>
              <RefreshCw size={14} className={detecting ? "animate-spin" : ""} /> {t("setup.refresh")}
            </Button>
          </div>

          {detecting && !env ? (
            <div className="flex justify-center py-12">
              <Spinner />
            </div>
          ) : !env ? (
            <div className="rounded-2xl border border-white/10 bg-white/3 px-4 py-10 text-center text-sm text-default-500">
              {t("setup.detect.empty")}
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-2xl bg-default-50 p-4">
                <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-default-500">
                  <Monitor size={12} /> {t("setup.detect.system")}
                </div>
                <StatusRow ok label={`${env.os} / ${env.arch} / ${env.num_cpu} CPU`} />
                <StatusRow ok={env.has_gpu} label={env.has_gpu ? `GPU · ${env.gpu_info || t("setup.connected")}` : `GPU · ${t("setup.unavailable")}`} />
                <StatusRow ok={env.has_docker} label={env.has_docker ? "Docker" : `Docker · ${t("setup.unavailable")}`} />
                <StatusRow ok label={`Data dir · ${env.data_dir || "data"}`} />
              </div>

              <div className="rounded-2xl bg-default-50 p-4">
                <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-default-500">
                  <Cpu size={12} /> {t("setup.detect.runtime")}
                </div>
                <StatusRow ok={env.has_node} label={env.has_node ? `Node.js ${env.node_version || ""}` : `Node.js · ${t("setup.unavailable")}`} />
                <StatusRow ok={env.has_python} label={env.has_python ? `Python ${env.python_version || ""}` : `Python · ${t("setup.unavailable")}`} />
                <StatusRow ok={env.has_ollama} label={env.has_ollama ? `Ollama (${env.ollama_models?.length || 0})` : `Ollama · ${t("setup.unavailable")}`} />
              </div>

              {env.providers?.length > 0 && (
                <div className="rounded-2xl bg-default-50 p-4 md:col-span-2">
                  <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-default-500">
                    <Server size={12} /> {t("setup.detect.providers")}
                  </div>
                  <div className="grid gap-2">
                    {env.providers.map((provider) => (
                      <StatusRow
                        key={`${provider.name}-${provider.model}`}
                        ok={provider.available}
                        label={`${provider.name}: ${provider.model || provider.base_url}${provider.latency ? ` · ${provider.latency}` : ""}${provider.error ? ` · ${provider.error}` : ""}`}
                      />
                    ))}
                  </div>
                </div>
              )}

              {env.components?.length > 0 && (
                <div className="rounded-2xl bg-default-50 p-4 md:col-span-2">
                  <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-default-500">
                    <Layers size={12} /> {t("setup.detect.components")}
                  </div>
                  <div className="grid gap-3">
                    {env.components.map((component) => {
                      const installed = component.installed;
                      const message = installMessage[component.id];
                      return (
                        <div key={component.id} className="rounded-2xl border border-white/8 bg-white/3 p-4">
                          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="text-sm font-medium">{component.name}</div>
                                <Chip size="sm" style={{ background: installed ? "rgba(34,197,94,0.12)" : "rgba(255,255,255,0.06)", color: installed ? "#22c55e" : "var(--yunque-text-muted)" }}>
                                  {installed ? t("setup.installed") : t("setup.notInstalled")}
                                </Chip>
                                {component.size && !installed && <span className="text-xs text-default-400">{component.size}</span>}
                              </div>
                              <p className="mt-1 text-sm text-default-500">{component.description}</p>
                              {installingId === component.id && installProgress && (
                                <div className="mt-3 space-y-1">
                                  <div className="flex items-center justify-between text-xs text-default-500">
                                    <span>{installProgress.detail}</span>
                                    <span>{Math.round(installProgress.percent)}%</span>
                                  </div>
                                  <ProgressBar aria-label="install-progress" value={installProgress.percent} maxValue={100} size="sm" />
                                </div>
                              )}
                              {message && <div className={`mt-2 text-xs ${message.ok ? "text-success" : "text-danger"}`}>{message.msg}</div>}
                            </div>
                            {!installed &&
                              (component.installable ? (
                                <Button size="sm" variant="outline" isDisabled={!!installingId} onPress={() => installComponent(component.id)}>
                                  {installingId === component.id ? t("setup.installing") : t("setup.install")}
                                </Button>
                              ) : (
                                <span className="text-xs text-default-400">{t("setup.manual")}</span>
                              ))}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {env.first_run && <div className="text-sm text-warning md:col-span-2">{t("setup.detect.firstRun")}</div>}
            </div>
          )}

          <div className="mt-6 flex items-center justify-between">
            <Button variant="ghost" onPress={() => setStep(STEP_CHOOSE)}>
              <ChevronLeft size={16} /> {t("setup.back")}
            </Button>
            <Button className="btn-accent" onPress={() => setStep(STEP_MODEL)}>
              {t("setup.next")} <ChevronRight size={16} />
            </Button>
          </div>
        </Card>
      )}

      {step === STEP_MODEL && (
        <Card className="p-6">
          <h2 className="mb-2 flex items-center gap-2 text-lg font-semibold">
            <Zap size={18} /> {t("setup.model.title")}
          </h2>
          <p className="mb-5 text-sm text-default-500">{t("setup.model.subtitle")}</p>

          <div className="space-y-4">
            <TextField aria-label="base-url">
              <Label>{t("setup.model.baseUrl")}</Label>
              <Input placeholder="https://api.openai.com/v1" value={baseURL} onChange={(e) => setBaseURL(e.target.value)} />
            </TextField>

            <TextField aria-label="api-key">
              <Label>{t("setup.model.apiKey")}</Label>
              <div className="flex items-center gap-2">
                <Input className="flex-1" type={showKey ? "text" : "password"} placeholder="sk-..." value={apiKey} onChange={(e) => setApiKey(e.target.value)} />
                <Button isIconOnly variant="ghost" size="sm" aria-label="toggle-api-key" onPress={() => setShowKey((value) => !value)}>
                  {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
                </Button>
              </div>
            </TextField>

            <TextField aria-label="model-name">
              <Label>{t("setup.model.name")}</Label>
              <Input placeholder="gpt-4.1-mini / deepseek-chat / qwen-plus" value={model} onChange={(e) => setModel(e.target.value)} />
            </TextField>

            <div className="flex flex-wrap items-center gap-3">
              <Button variant="ghost" size="sm" isDisabled={!baseURL || testing} onPress={testLLM}>
                <Server size={14} className={testing ? "animate-pulse" : ""} /> {testing ? t("setup.model.testing") : t("setup.model.test")}
              </Button>
              {testResult && (
                <Chip size="sm" style={{ background: testResult.ok ? "rgba(34,197,94,0.12)" : "rgba(239,68,68,0.12)", color: testResult.ok ? "#22c55e" : "#ef4444" }}>
                  {testResult.msg}
                </Chip>
              )}
            </div>
          </div>

          <div className="mt-6 flex justify-between">
            <Button variant="ghost" onPress={() => setStep(STEP_DETECT)}>
              <ChevronLeft size={16} /> {t("setup.back")}
            </Button>
            <Button className="btn-accent" isDisabled={!baseURL || !model} onPress={() => setStep(STEP_TEMPLATE)}>
              {t("setup.next")} <ChevronRight size={16} />
            </Button>
          </div>
        </Card>
      )}

      {step === STEP_TEMPLATE && (
        <Card className="p-6">
          <h2 className="mb-2 flex items-center gap-2 text-lg font-semibold">
            <Layers size={18} /> {t("setup.template.title")}
          </h2>
          <p className="mb-5 text-sm text-default-500">{t("setup.template.subtitle")}</p>

          {templates.length === 0 ? (
            <div className="flex justify-center py-10">
              <Spinner />
            </div>
          ) : (
            <div className="grid gap-3 md:grid-cols-2">
              {templates.map((template) => {
                const Icon = templateIcons[template.id] || Bot;
                const selected = template.id === selectedTpl;
                return (
                  <button
                    key={template.id}
                    type="button"
                    onClick={() => setSelectedTpl(template.id)}
                    className="rounded-2xl border-2 p-4 text-left transition cursor-pointer"
                    style={{
                      borderColor: selected ? "var(--yunque-accent)" : "var(--yunque-border)",
                      background: selected ? "rgba(0,111,238,0.08)" : "transparent",
                    }}
                  >
                    <div className="mb-2 flex items-center gap-2">
                      <Icon size={18} className={selected ? "text-primary" : "text-default-500"} />
                      <span className="text-sm font-medium">{template.name}</span>
                      <Chip size="sm" className="ml-auto">
                        {categoryLabels[template.category] || template.category}
                      </Chip>
                    </div>
                    <p className="text-sm text-default-500">{template.description}</p>
                    <div className="mt-3 flex flex-wrap gap-1">
                      {template.skills.slice(0, 4).map((skill) => (
                        <Chip key={skill} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-text-secondary)" }}>
                          {skill}
                        </Chip>
                      ))}
                      {template.skills.length > 4 && <Chip size="sm">+{template.skills.length - 4}</Chip>}
                    </div>
                  </button>
                );
              })}
            </div>
          )}

          <div className="mt-6 flex justify-between">
            <Button variant="ghost" onPress={() => setStep(STEP_MODEL)}>
              <ChevronLeft size={16} /> {t("setup.back")}
            </Button>
            <Button className="btn-accent" isDisabled={!selectedTpl} onPress={() => setStep(STEP_DONE)}>
              {t("setup.step.done")} <Rocket size={16} />
            </Button>
          </div>
        </Card>
      )}

      {step === STEP_DONE && (
        <Card className="p-8 text-center">
          {applying ? (
            <div className="flex flex-col items-center gap-4 py-12">
              <Spinner size="lg" />
              <p className="text-sm text-default-500">Saving setup...</p>
            </div>
          ) : (
            <div className="flex flex-col items-center gap-4 py-8">
              <CheckCircle2 size={48} className="text-success" />
              <h2 className="text-2xl font-semibold">{t("setup.done.title")}</h2>
              <p className="max-w-xl text-sm text-default-500">{applyMessage || t("setup.done.subtitle")}</p>
              <div className="mt-4 flex flex-wrap justify-center gap-3">
                <Button variant="ghost" onPress={() => router.push("/settings")}>
                  <Settings size={16} /> {t("setup.done.settings")}
                </Button>
                <Button className="btn-accent" onPress={() => router.push("/chat")}>
                  <Rocket size={16} /> {t("setup.done.login")}
                </Button>
              </div>
            </div>
          )}
        </Card>
      )}
    </div>
  );
}

function StatusRow({ ok, label }: { ok: boolean; label: string }) {
  return (
    <div className="flex items-center gap-2 text-sm">
      {ok ? <CheckCircle2 size={16} className="shrink-0 text-success" /> : <XCircle size={16} className="shrink-0 text-default-400" />}
      <span>{label}</span>
    </div>
  );
}

function ChoiceCard({
  icon: Icon,
  accent,
  title,
  tag,
  desc,
  cta,
  onPress,
}: {
  icon: React.ElementType;
  accent: string;
  title: string;
  tag: string;
  desc: string;
  cta: string;
  onPress: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onPress}
      className="group flex flex-col gap-4 rounded-2xl border border-white/10 bg-white/3 p-5 text-left transition hover:border-white/25 hover:bg-white/[0.06] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
    >
      <div className="flex items-start justify-between gap-3">
        <div
          className="flex size-10 items-center justify-center rounded-xl"
          style={{ background: `${accent}1f`, color: accent }}
        >
          <Icon size={20} />
        </div>
        <span
          className="rounded-full px-2 py-1 text-[11px] font-medium"
          style={{ background: `${accent}1a`, color: accent }}
        >
          {tag}
        </span>
      </div>
      <div className="space-y-1.5">
        <div className="text-base font-semibold">{title}</div>
        <p className="text-sm text-default-500">{desc}</p>
      </div>
      <div className="mt-auto flex items-center gap-1.5 text-sm font-medium" style={{ color: accent }}>
        {cta}
        <ChevronRight size={14} className="transition group-hover:translate-x-0.5" />
      </div>
    </button>
  );
}
