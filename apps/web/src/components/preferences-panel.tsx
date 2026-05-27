"use client";

import { useState } from "react";
import { Button, Chip } from "@heroui/react";
import {
  Download,
  Upload,
  RotateCcw,
  Check,
  AlertTriangle,
  Star,
  Clock,
  Zap,
  Eye,
  EyeOff,
  Bell,
  BellOff,
  Volume2,
  VolumeX,
  Save,
  Trash2,
} from "lucide-react";
import { useUserPreferences } from "@/hooks/use-user-preferences";
import { exportPreferences, importPreferences } from "@/lib/user-preferences";
import { showToast } from "@/components/toast-provider";

export function PreferencesPanel() {
  const { preferences, updatePreferences, resetPreferences } = useUserPreferences();
  const [importing, setImporting] = useState(false);

  const handleExport = () => {
    try {
      const json = exportPreferences();
      const blob = new Blob([json], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `yunque-preferences-${Date.now()}.json`;
      a.click();
      URL.revokeObjectURL(url);
      showToast("偏好设置已导出", "success");
    } catch (error) {
      showToast("导出失败", "error");
      console.error(error);
    }
  };

  const handleImport = () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "application/json";
    input.onchange = async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;

      setImporting(true);
      try {
        const text = await file.text();
        importPreferences(text);
        showToast("偏好设置已导入", "success");
        window.location.reload();
      } catch (error) {
        showToast("导入失败：文件格式无效", "error");
        console.error(error);
      } finally {
        setImporting(false);
      }
    };
    input.click();
  };

  const handleReset = () => {
    if (confirm("确定要重置所有偏好设置吗？此操作无法撤销。")) {
      resetPreferences();
      showToast("偏好设置已重置", "success");
      window.location.reload();
    }
  };

  const toggleBehavioral = (key: keyof typeof preferences.behavioral) => {
    updatePreferences("behavioral", {
      [key]: !preferences.behavioral[key],
    });
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-4)" }}>
      {/* Header */}
      <div>
        <h2 style={{ fontSize: "var(--text-lg)", fontWeight: 600, marginBottom: "var(--sp-2)" }}>
          个性化偏好
        </h2>
        <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>
          管理您的界面偏好、工作流习惯和行为设置
        </p>
      </div>

      {/* Stats */}
      <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: "var(--sp-3)" }}>
          <StatCard
            icon={<Clock size={16} />}
            label="最近访问"
            value={preferences.navigation.recentPages.length}
            color="var(--yunque-accent)"
          />
          <StatCard
            icon={<Star size={16} />}
            label="收藏项"
            value={preferences.workflow.favoriteSkills.length + preferences.workflow.favoriteWorkflows.length}
            color="var(--yunque-warning)"
          />
          <StatCard
            icon={<Zap size={16} />}
            label="快捷操作"
            value={preferences.workflow.quickActions.length}
            color="var(--yunque-success)"
          />
        </div>
      </div>

      {/* Behavioral Settings */}
      <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
        <h3 style={{ fontSize: "var(--text-md)", fontWeight: 600, marginBottom: "var(--sp-3)" }}>
          行为设置
        </h3>
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
          <PreferenceToggle
            icon={preferences.behavioral.autoSaveEnabled ? <Save size={16} /> : <Save size={16} />}
            label="自动保存"
            description="自动保存草稿和未完成的工作"
            enabled={preferences.behavioral.autoSaveEnabled}
            onToggle={() => toggleBehavioral("autoSaveEnabled")}
          />
          <PreferenceToggle
            icon={preferences.behavioral.notificationsEnabled ? <Bell size={16} /> : <BellOff size={16} />}
            label="通知"
            description="显示系统通知和提醒"
            enabled={preferences.behavioral.notificationsEnabled}
            onToggle={() => toggleBehavioral("notificationsEnabled")}
          />
          <PreferenceToggle
            icon={preferences.behavioral.soundEnabled ? <Volume2 size={16} /> : <VolumeX size={16} />}
            label="声音"
            description="播放操作反馈音效"
            enabled={preferences.behavioral.soundEnabled}
            onToggle={() => toggleBehavioral("soundEnabled")}
          />
          <PreferenceToggle
            icon={preferences.behavioral.analyticsEnabled ? <Eye size={16} /> : <EyeOff size={16} />}
            label="使用分析"
            description="帮助改进产品体验"
            enabled={preferences.behavioral.analyticsEnabled}
            onToggle={() => toggleBehavioral("analyticsEnabled")}
          />
        </div>
      </div>

      {/* Recent Pages */}
      {preferences.navigation.recentPages.length > 0 && (
        <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sp-3)" }}>
            <h3 style={{ fontSize: "var(--text-md)", fontWeight: 600 }}>最近访问</h3>
            <Button
              size="sm"
              variant="ghost"
              onPress={() => updatePreferences("navigation", { recentPages: [] })}
            >
              <Trash2 size={12} />
              清空
            </Button>
          </div>
          <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
            {preferences.navigation.recentPages.slice(0, 10).map((page) => (
              <a
                key={page.path}
                href={page.path}
                style={{
                  fontSize: "var(--text-xs)",
                  padding: "4px 10px",
                  borderRadius: "var(--radius-sm)",
                  background: "var(--yunque-bg-muted)",
                  color: "var(--yunque-text-secondary)",
                  textDecoration: "none",
                  transition: "all 0.15s ease",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = "var(--yunque-accent-muted)";
                  e.currentTarget.style.color = "var(--yunque-accent)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = "var(--yunque-bg-muted)";
                  e.currentTarget.style.color = "var(--yunque-text-secondary)";
                }}
              >
                {page.label}
              </a>
            ))}
          </div>
        </div>
      )}

      {/* Favorites */}
      {(preferences.workflow.favoriteSkills.length > 0 || preferences.workflow.favoriteWorkflows.length > 0) && (
        <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
          <h3 style={{ fontSize: "var(--text-md)", fontWeight: 600, marginBottom: "var(--sp-3)" }}>
            收藏
          </h3>
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
            {preferences.workflow.favoriteSkills.length > 0 && (
              <div>
                <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginBottom: "var(--sp-1)" }}>
                  技能
                </div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
                  {preferences.workflow.favoriteSkills.map((skill) => (
                    <Chip key={skill} size="sm" variant="soft">
                      <Star size={10} style={{ marginRight: 4 }} />
                      {skill}
                    </Chip>
                  ))}
                </div>
              </div>
            )}
            {preferences.workflow.favoriteWorkflows.length > 0 && (
              <div>
                <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginBottom: "var(--sp-1)" }}>
                  工作流
                </div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--sp-2)" }}>
                  {preferences.workflow.favoriteWorkflows.map((workflow) => (
                    <Chip key={workflow} size="sm" variant="soft">
                      <Star size={10} style={{ marginRight: 4 }} />
                      {workflow}
                    </Chip>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
        <h3 style={{ fontSize: "var(--text-md)", fontWeight: 600, marginBottom: "var(--sp-3)" }}>
          数据管理
        </h3>
        <div style={{ display: "flex", gap: "var(--sp-2)", flexWrap: "wrap" }}>
          <Button size="sm" variant="outline" onPress={handleExport}>
            <Download size={13} />
            导出偏好
          </Button>
          <Button size="sm" variant="outline" onPress={handleImport} isPending={importing}>
            <Upload size={13} />
            导入偏好
          </Button>
          <Button size="sm" variant="outline" onPress={handleReset} style={{ color: "var(--yunque-danger)" }}>
            <RotateCcw size={13} />
            重置所有
          </Button>
        </div>
        <p style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)", marginTop: "var(--sp-2)" }}>
          最后更新：{new Date(preferences.lastUpdated).toLocaleString("zh-CN")}
        </p>
      </div>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  color,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  color: string;
}) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--sp-2)",
        padding: "var(--sp-3)",
        borderRadius: "var(--radius-md)",
        background: "var(--yunque-bg-muted)",
      }}
    >
      <div style={{ color, flexShrink: 0 }}>{icon}</div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>{label}</div>
        <div style={{ fontSize: "var(--text-lg)", fontWeight: 600, color: "var(--yunque-text)" }}>{value}</div>
      </div>
    </div>
  );
}

function PreferenceToggle({
  icon,
  label,
  description,
  enabled,
  onToggle,
}: {
  icon: React.ReactNode;
  label: string;
  description: string;
  enabled: boolean;
  onToggle: () => void;
}) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--sp-3)",
        padding: "var(--sp-3)",
        borderRadius: "var(--radius-md)",
        background: "var(--yunque-bg-muted)",
        cursor: "pointer",
        transition: "background 0.15s ease",
      }}
      onClick={onToggle}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = "var(--yunque-bg-hover)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = "var(--yunque-bg-muted)";
      }}
    >
      <div style={{ color: enabled ? "var(--yunque-accent)" : "var(--yunque-text-muted)", flexShrink: 0 }}>
        {icon}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: "var(--text-sm)", fontWeight: 500, color: "var(--yunque-text)" }}>{label}</div>
        <div style={{ fontSize: "var(--text-xs)", color: "var(--yunque-text-muted)" }}>{description}</div>
      </div>
      <div
        style={{
          width: 40,
          height: 22,
          borderRadius: 11,
          background: enabled ? "var(--yunque-accent)" : "var(--yunque-border)",
          position: "relative",
          transition: "background 0.2s ease",
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: 18,
            height: 18,
            borderRadius: "50%",
            background: "#fff",
            position: "absolute",
            top: 2,
            left: enabled ? 20 : 2,
            transition: "left 0.2s ease",
          }}
        />
      </div>
    </div>
  );
}
