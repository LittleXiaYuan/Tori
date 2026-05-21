"use client";

import { useState } from "react";
import { api, type EmotionHistoryEntry } from "@/lib/api";
import { Card, Button, Chip, Select, Label, ListBox, Table, TextField, Input } from "@heroui/react";
import EmptyState from "@/components/empty-state";
import { SmilePlus, RefreshCw } from "lucide-react";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";

const emotionColors: Record<string, string> = {
  happy: "#facc15", sad: "#60a5fa", angry: "#f87171", neutral: "#a1a1aa",
  surprised: "#c084fc", fearful: "#fb923c", disgusted: "#86efac", loving: "#f472b6",
};
const emotionEmoji: Record<string, string> = {
  happy: "[+]", sad: "[-]", angry: "[!]", neutral: "[=]",
  surprised: "[?]", fearful: "[~]", disgusted: "[x]", loving: "[<3]",
};

export default function EmotionsPage() {
  const [limit, setLimit] = useState(200);
  const [sessionFilter, setSessionFilter] = useState("");

  const { data, loading, refresh } = useApiData(
    async () => {
      const res = await api.getEmotionHistory({ session_id: sessionFilter || undefined, limit });
      return { entries: res.entries || [] as EmotionHistoryEntry[], summary: res.summary || {} as Record<string, number>, total: res.total || 0 };
    },
    { entries: [] as EmotionHistoryEntry[], summary: {} as Record<string, number>, total: 0 },
    [limit, sessionFilter],
  );
  const { entries, summary, total } = data;

  const maxCount = Math.max(...Object.values(summary), 1);

  const hourlyTrend = entries.reduce<Record<string, Record<string, number>>>((acc, e) => {
    const hour = e.timestamp.slice(0, 13);
    if (!acc[hour]) acc[hour] = {};
    acc[hour][e.emotion] = (acc[hour][e.emotion] || 0) + 1;
    return acc;
  }, {});
  const trendHours = Object.keys(hourlyTrend).sort();

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader icon={<SmilePlus size={20} />} title="情感历史" onRefresh={refresh} />

      {/* Filters */}
      <Card>
        <div className="flex gap-3 items-end flex-wrap p-4">
          <div>
            <TextField
              value={sessionFilter}
              onChange={(v) => setSessionFilter(v)}
              className="w-[200px]"
            >
              <Label>Session ID</Label>
              <Input placeholder="全部 session" />
            </TextField>
          </div>
          <div>
            <Select
              selectedKey={String(limit)}
              onSelectionChange={(key) => setLimit(Number(key))}
              className="w-[120px]"
              placeholder="数量"
            >
              <Label>数量</Label>
              <Select.Trigger>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover>
                <ListBox>
                  <ListBox.Item id="50" textValue="50">50<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="100" textValue="100">100<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="200" textValue="200">200<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="500" textValue="500">500<ListBox.ItemIndicator /></ListBox.Item>
                </ListBox>
              </Select.Popover>
            </Select>
          </div>
        </div>
      </Card>

      {/* Summary + Trend side by side */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">

      {/* Summary Bar Chart */}
      <Card className="section-card p-5">
        <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--yunque-text-muted)" }}>
          情感分布 ({total} events)
        </div>
        {Object.keys(summary).length === 0 ? (
          <div className="text-sm text-center py-8" style={{ color: "var(--yunque-text-muted)" }}>暂无情感数据</div>
        ) : (
          <div className="space-y-2">
            {Object.entries(summary).sort(([, a], [, b]) => b - a).map(([emo, count]) => (
              <div key={emo} className="flex items-center gap-3">
                <span className="text-base w-6 text-center">{emotionEmoji[emo] || "•"}</span>
                <span className="text-sm w-20 capitalize" style={{ color: "var(--yunque-text)" }}>{emo}</span>
                <div className="flex-1 h-5 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.04)" }}>
                  <div className="h-full rounded-full transition-all duration-500"
                    style={{ width: `${(count / maxCount) * 100}%`, background: emotionColors[emo] || "var(--yunque-accent)" }} />
                </div>
                <span className="text-sm tabular-nums w-10 text-right" style={{ color: "var(--yunque-text-muted)" }}>{count}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Hourly Trend */}
      {trendHours.length > 1 ? (
        <Card className="section-card p-5">
          <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--yunque-text-muted)" }}>
            每小时趋势
          </div>
          <div className="flex gap-1 items-end overflow-x-auto pb-2" style={{ minHeight: 120 }}>
            {trendHours.map((hour) => {
              const hourData = hourlyTrend[hour];
              const maxHourTotal = Math.max(...trendHours.map((h) => Object.values(hourlyTrend[h]).reduce((a, b) => a + b, 0)), 1);
              return (
                <div key={hour} className="flex flex-col items-center gap-1" style={{ minWidth: 32 }}>
                  <div className="flex flex-col-reverse rounded overflow-hidden" style={{ width: 20, height: 80 }}>
                    {Object.entries(hourData).map(([emo, count]) => (
                      <div key={emo} style={{ height: `${(count / maxHourTotal) * 80}px`, background: emotionColors[emo] || "var(--yunque-accent)" }} />
                    ))}
                  </div>
                  <span className="text-[10px] tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>{hour.slice(11, 13)}h</span>
                </div>
              );
            })}
          </div>
          <div className="flex gap-3 mt-3 flex-wrap">
            {Object.keys(summary).map((emo) => (
              <div key={emo} className="flex items-center gap-1">
                <div className="w-3 h-3 rounded-sm" style={{ background: emotionColors[emo] || "var(--yunque-accent)" }} />
                <span className="text-xs capitalize" style={{ color: "var(--yunque-text-muted)" }}>{emo}</span>
              </div>
            ))}
          </div>
        </Card>
      ) : (
        <Card className="section-card p-5 flex items-center justify-center">
          <div className="text-sm text-center" style={{ color: "var(--yunque-text-muted)" }}>数据不足，无法显示趋势</div>
        </Card>
      )}

      </div>{/* end 2-col grid */}

      {/* Recent Events Table */}
      <Card className="section-card p-5">
        <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--yunque-text-muted)" }}>
          最近事件
        </div>
        {entries.length === 0 ? (
          <EmptyState icon={<SmilePlus size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无数据" description="情感事件将在此显示" />
        ) : (
          <Table>
            <Table.ScrollContainer>
              <Table.Content aria-label="情感事件" className="min-w-[600px]">
                <Table.Header>
                  <Table.Column isRowHeader>时间</Table.Column>
                  <Table.Column>情感</Table.Column>
                  <Table.Column>置信度</Table.Column>
                  <Table.Column>来源</Table.Column>
                  <Table.Column>Session</Table.Column>
                </Table.Header>
                <Table.Body>
                  {entries.slice(-50).reverse().map((e, i) => (
                    <Table.Row key={i}>
                      <Table.Cell>
                        <span className="tabular-nums text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                          {new Date(e.timestamp).toLocaleString()}
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="inline-flex items-center gap-1" style={{ color: "var(--yunque-text)" }}>
                          {emotionEmoji[e.emotion] || "•"}
                          <span className="capitalize">{e.emotion}</span>
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="tabular-nums" style={{ color: "var(--yunque-text)" }}>{(e.confidence * 100).toFixed(0)}%</span>
                      </Table.Cell>
                      <Table.Cell>
                        <span style={{ color: "var(--yunque-text-muted)" }}>{e.source}</span>
                      </Table.Cell>
                      <Table.Cell>
                        <span className="text-xs font-mono truncate max-w-[120px]" style={{ color: "var(--yunque-text-muted)" }}>{e.session_id}</span>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Card>
    </div>
  );
}
