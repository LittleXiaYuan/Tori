"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api, type GraphEntity, type GraphRelation, type GraphStats } from "@/lib/api";
import { Card, Button, Chip, Spinner, TextField, Input } from "@heroui/react";
import EmptyState from "@/components/empty-state";
import { Share2, RefreshCw, Search, ZoomIn, ZoomOut, Maximize2, Trash2, Info } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import PageHeader from "@/components/page-header";

// ── Color palette for entity types ──
const typeColors: Record<string, string> = {
  person: "#3b82f6",
  place: "#22c55e",
  concept: "#a78bfa",
  event: "#f97316",
  project: "#06b6d4",
  skill: "#eab308",
  preference: "#ec4899",
  tool: "#64748b",
};
function typeColor(t: string): string {
  return typeColors[t.toLowerCase()] || "var(--yunque-accent)";
}

// ── Simple force-directed layout simulation ──
interface NodePos {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  radius: number;
  entity: GraphEntity;
  pinned?: boolean;
}

function initLayout(entities: GraphEntity[], w: number, h: number): NodePos[] {
  const cx = w / 2, cy = h / 2;
  return entities.map((e, i) => {
    const angle = (2 * Math.PI * i) / entities.length;
    const r = Math.min(w, h) * 0.3;
    return {
      id: e.id,
      x: cx + r * Math.cos(angle) + (Math.random() - 0.5) * 40,
      y: cy + r * Math.sin(angle) + (Math.random() - 0.5) * 40,
      vx: 0, vy: 0,
      radius: Math.max(18, Math.min(36, 14 + (e.mentions ?? 1) * 2)),
      entity: e,
    };
  });
}

function simulate(nodes: NodePos[], edges: GraphRelation[], w: number, h: number, steps = 1) {
  const k = 80; // ideal distance
  const gravity = 0.01;
  const damping = 0.85;
  const cx = w / 2, cy = h / 2;

  for (let step = 0; step < steps; step++) {
    // Repulsion (all pairs)
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        let dx = nodes[j].x - nodes[i].x;
        let dy = nodes[j].y - nodes[i].y;
        let dist = Math.sqrt(dx * dx + dy * dy) || 1;
        let force = (k * k) / dist;
        let fx = (dx / dist) * force;
        let fy = (dy / dist) * force;
        if (!nodes[i].pinned) { nodes[i].vx -= fx * 0.02; nodes[i].vy -= fy * 0.02; }
        if (!nodes[j].pinned) { nodes[j].vx += fx * 0.02; nodes[j].vy += fy * 0.02; }
      }
    }
    // Attraction (edges)
    const nodeMap = new Map(nodes.map(n => [n.id, n]));
    for (const e of edges) {
      const a = nodeMap.get(e.from_id);
      const b = nodeMap.get(e.to_id);
      if (!a || !b) continue;
      let dx = b.x - a.x;
      let dy = b.y - a.y;
      let dist = Math.sqrt(dx * dx + dy * dy) || 1;
      let force = (dist - k) * 0.005 * (e.weight ?? 0.5);
      let fx = (dx / dist) * force;
      let fy = (dy / dist) * force;
      if (!a.pinned) { a.vx += fx; a.vy += fy; }
      if (!b.pinned) { b.vx -= fx; b.vy -= fy; }
    }
    // Gravity toward center
    for (const n of nodes) {
      if (n.pinned) continue;
      n.vx += (cx - n.x) * gravity;
      n.vy += (cy - n.y) * gravity;
      n.x += n.vx;
      n.y += n.vy;
      n.vx *= damping;
      n.vy *= damping;
      // Bounds
      n.x = Math.max(n.radius, Math.min(w - n.radius, n.x));
      n.y = Math.max(n.radius, Math.min(h - n.radius, n.y));
    }
  }
}

export default function GraphPage() {
  const [entities, setEntities] = useState<GraphEntity[]>([]);
  const [relations, setRelations] = useState<GraphRelation[]>([]);
  const [stats, setStats] = useState<GraphStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<GraphEntity | null>(null);
  const [selectedRelations, setSelectedRelations] = useState<GraphRelation[]>([]);
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });

  const svgRef = useRef<SVGSVGElement>(null);
  const nodesRef = useRef<NodePos[]>([]);
  const animRef = useRef<number>(0);
  const [tick, setTick] = useState(0);
  const dragRef = useRef<{ nodeId: string; startX: number; startY: number } | null>(null);
  const panStartRef = useRef<{ x: number; y: number; panX: number; panY: number } | null>(null);

  const WIDTH = 900;
  const HEIGHT = 600;

  const load = useCallback(async () => {
    try {
      const [entRes, relRes, st] = await Promise.all([
        api.graphEntities(200),
        api.graphRelations(),
        api.graphStats(),
      ]);
      setEntities(entRes.entities || []);
      setRelations(relRes.relations || []);
      setStats(st);
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Initialize layout when entities change
  useEffect(() => {
    if (entities.length === 0) return;
    nodesRef.current = initLayout(entities, WIDTH, HEIGHT);
    // Run initial simulation
    simulate(nodesRef.current, relations, WIDTH, HEIGHT, 100);
    setTick(t => t + 1);

    // Continue simulation in animation loop
    let frame = 0;
    const maxFrames = 200;
    const step = () => {
      if (frame >= maxFrames) return;
      simulate(nodesRef.current, relations, WIDTH, HEIGHT, 2);
      setTick(t => t + 1);
      frame++;
      animRef.current = requestAnimationFrame(step);
    };
    animRef.current = requestAnimationFrame(step);
    return () => cancelAnimationFrame(animRef.current);
  }, [entities, relations]);

  const handleNodeClick = async (entity: GraphEntity) => {
    setSelected(entity);
    try {
      const res = await api.graphRelations(entity.id);
      setSelectedRelations(res.relations || []);
    } catch { setSelectedRelations([]); }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.graphDeleteEntity(id);
      setEntities(prev => prev.filter(e => e.id !== id));
      setRelations(prev => prev.filter(r => r.from_id !== id && r.to_id !== id));
      if (selected?.id === id) { setSelected(null); setSelectedRelations([]); }
      showToast("实体已删除", "success");
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
  };

  const handleSearch = async () => {
    if (!search.trim()) { load(); return; }
    setLoading(true);
    try {
      const res = await api.graphEntities(200);
      const q = search.toLowerCase();
      const filtered = (res.entities || []).filter(e =>
        e.name.toLowerCase().includes(q) || e.type.toLowerCase().includes(q)
      );
      setEntities(filtered);
      // Also filter relations to only those between visible entities
      const ids = new Set(filtered.map(e => e.id));
      const relRes = await api.graphRelations();
      setRelations((relRes.relations || []).filter(r => ids.has(r.from_id) && ids.has(r.to_id)));
    } catch { /* */ }
    finally { setLoading(false); }
  };

  // Drag handlers
  const onNodeMouseDown = (e: React.MouseEvent, nodeId: string) => {
    e.stopPropagation();
    const node = nodesRef.current.find(n => n.id === nodeId);
    if (node) {
      node.pinned = true;
      dragRef.current = { nodeId, startX: e.clientX, startY: e.clientY };
    }
  };

  const onSvgMouseMove = (e: React.MouseEvent) => {
    if (dragRef.current) {
      const node = nodesRef.current.find(n => n.id === dragRef.current!.nodeId);
      if (node) {
        node.x += (e.movementX) / zoom;
        node.y += (e.movementY) / zoom;
        setTick(t => t + 1);
      }
    } else if (panStartRef.current) {
      setPan({ x: panStartRef.current.panX + e.clientX - panStartRef.current.x, y: panStartRef.current.panY + e.clientY - panStartRef.current.y });
    }
  };

  const onSvgMouseUp = () => {
    if (dragRef.current) {
      const node = nodesRef.current.find(n => n.id === dragRef.current!.nodeId);
      if (node) node.pinned = false;
      dragRef.current = null;
    }
    panStartRef.current = null;
  };

  const onSvgMouseDown = (e: React.MouseEvent) => {
    if ((e.target as Element).tagName === "svg" || (e.target as Element).tagName === "rect") {
      panStartRef.current = { x: e.clientX, y: e.clientY, panX: pan.x, panY: pan.y };
    }
  };

  const entityName = (id: string) => entities.find(e => e.id === id)?.name || id;

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  const nodes = nodesRef.current;
  const nodeMap = new Map(nodes.map(n => [n.id, n]));

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader icon={<Share2 size={20} />} title="知识图谱" onRefresh={() => { setLoading(true); load(); }} />

      {/* Stats */}
      {stats && (
        <div className="kpi-grid">
          {[
            { label: "实体", value: stats.entities, color: "var(--yunque-accent)" },
            { label: "关系", value: stats.relations, color: "#a78bfa" },
            { label: "实体类型", value: Object.keys(stats.entity_types || {}).length, color: "#22c55e" },
            { label: "关系类型", value: Object.keys(stats.relation_types || {}).length, color: "#f97316" },
          ].map((s) => (
            <Card key={s.label} className="section-card p-4 text-center">
              <div className="kpi-label mb-1">{s.label}</div>
              <div className="kpi-value" style={{ color: s.color }}>{s.value}</div>
            </Card>
          ))}
        </div>
      )}

      {/* Search */}
      <div className="flex items-center gap-2">
        <div className="flex-1">
          <TextField value={search} onChange={setSearch}>
            <Input placeholder="搜索实体名称或类型?.." onKeyDown={(e: React.KeyboardEvent) => e.key === "Enter" && handleSearch()} />
          </TextField>
        </div>
        <Button size="sm" onPress={handleSearch} className="btn-accent">
          <Search size={14} /> 搜索
        </Button>
      </div>

      {/* Type legend */}
      {stats && Object.keys(stats.entity_types || {}).length > 0 && (
        <div className="flex flex-wrap gap-2">
          {Object.entries(stats.entity_types).map(([type, count]) => (
            <Chip key={type} size="sm" style={{ background: `${typeColor(type)}20`, color: typeColor(type) }}>
              {type} ({count})
            </Chip>
          ))}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* ── Graph Canvas ── */}
        <div className="lg:col-span-2">
          <Card className="section-card overflow-hidden" style={{ position: "relative" }}>
            {entities.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  icon={<Share2 size={24} style={{ color: "var(--yunque-accent)" }} />}
                  title="暂无知识图谱数据"
                  description="与云雀对话后，关键实体和关系将自动提取到知识图谱中"
                />
              </div>
            ) : (
              <>
                {/* Zoom controls */}
                <div className="absolute top-3 right-3 z-10 flex flex-col gap-1">
                  <Button isIconOnly aria-label="放大" size="sm" variant="ghost" onPress={() => setZoom(z => Math.min(2, z + 0.2))}>
                    <ZoomIn size={14} />
                  </Button>
                  <Button isIconOnly aria-label="缩小" size="sm" variant="ghost" onPress={() => setZoom(z => Math.max(0.3, z - 0.2))}>
                    <ZoomOut size={14} />
                  </Button>
                  <Button isIconOnly aria-label="全屏" size="sm" variant="ghost" onPress={() => { setZoom(1); setPan({ x: 0, y: 0 }); }}>
                    <Maximize2 size={14} />
                  </Button>
                </div>

                <svg
                  ref={svgRef}
                  width="100%"
                  viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
                  style={{ cursor: panStartRef.current ? "grabbing" : "grab", minHeight: 400, background: "var(--yunque-bg)" }}
                  onMouseDown={onSvgMouseDown}
                  onMouseMove={onSvgMouseMove}
                  onMouseUp={onSvgMouseUp}
                  onMouseLeave={onSvgMouseUp}
                >
                  <g transform={`translate(${pan.x / zoom}, ${pan.y / zoom}) scale(${zoom})`}>
                    {/* Edges */}
                    {relations.map(rel => {
                      const from = nodeMap.get(rel.from_id);
                      const to = nodeMap.get(rel.to_id);
                      if (!from || !to) return null;
                      const isHighlight = selected && (rel.from_id === selected.id || rel.to_id === selected.id);
                      return (
                        <g key={rel.id}>
                          <line
                            x1={from.x} y1={from.y} x2={to.x} y2={to.y}
                            stroke={isHighlight ? "var(--yunque-accent)" : "rgba(255,255,255,0.08)"}
                            strokeWidth={isHighlight ? 2 : 1}
                            strokeDasharray={isHighlight ? undefined : "4 2"}
                          />
                          {/* Label */}
                          <text
                            x={(from.x + to.x) / 2}
                            y={(from.y + to.y) / 2 - 4}
                            textAnchor="middle"
                            fill={isHighlight ? "var(--yunque-accent)" : "var(--yunque-text-muted)"}
                            fontSize={9}
                            opacity={isHighlight ? 1 : 0.5}
                          >
                            {rel.type}
                          </text>
                        </g>
                      );
                    })}

                    {/* Nodes */}
                    {nodes.map(n => {
                      const isSelected = selected?.id === n.id;
                      const color = typeColor(n.entity.type);
                      return (
                        <g
                          key={n.id}
                          style={{ cursor: "pointer" }}
                          onMouseDown={(e) => onNodeMouseDown(e, n.id)}
                          onClick={() => handleNodeClick(n.entity)}
                        >
                          {/* Glow for selected */}
                          {isSelected && (
                            <circle cx={n.x} cy={n.y} r={n.radius + 6} fill="none" stroke={color} strokeWidth={2} opacity={0.4} />
                          )}
                          {/* Node circle */}
                          <circle
                            cx={n.x} cy={n.y} r={n.radius}
                            fill={`${color}30`}
                            stroke={color}
                            strokeWidth={isSelected ? 2.5 : 1.5}
                          />
                          {/* Label */}
                          <text
                            x={n.x} y={n.y + 1}
                            textAnchor="middle"
                            dominantBaseline="middle"
                            fill="var(--yunque-text)"
                            fontSize={n.radius > 24 ? 11 : 9}
                            fontWeight={isSelected ? 600 : 400}
                          >
                            {n.entity.name.length > 8 ? n.entity.name.slice(0, 7) + "…" : n.entity.name}
                          </text>
                          {/* Type badge below */}
                          <text
                            x={n.x} y={n.y + n.radius + 12}
                            textAnchor="middle"
                            fill="var(--yunque-text-muted)"
                            fontSize={8}
                          >
                            {n.entity.type}
                          </text>
                        </g>
                      );
                    })}
                  </g>
                </svg>
              </>
            )}
          </Card>
        </div>

        {/* ── Sidebar: Entity Detail ── */}
        <div className="space-y-4">
          {selected ? (
            <>
              <Card className="section-card p-5">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full" style={{ background: typeColor(selected.type) }} />
                    <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{selected.name}</span>
                  </div>
                  <Button isIconOnly aria-label="删除" size="sm" variant="ghost" onPress={() => handleDelete(selected.id)}
                    style={{ color: "var(--yunque-danger)" }}>
                    <Trash2 size={12} />
                  </Button>
                </div>
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Chip size="sm" style={{ background: `${typeColor(selected.type)}20`, color: typeColor(selected.type) }}>{selected.type}</Chip>
                    {selected.mentions && (
                      <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>提及 {selected.mentions} 次</span>
                    )}
                  </div>
                  {selected.properties && Object.keys(selected.properties).length > 0 && (
                    <div className="space-y-1 mt-2">
                      <div className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>属性</div>
                      {Object.entries(selected.properties).map(([k, v]) => (
                        <div key={k} className="flex items-baseline gap-2 text-xs">
                          <span style={{ color: "var(--yunque-text-muted)" }}>{k}:</span>
                          <span style={{ color: "var(--yunque-text)" }}>{v}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                    创建于 {new Date(selected.created_at).toLocaleString()}
                  </div>
                </div>
              </Card>

              {/* Relations */}
              <Card className="section-card p-5">
                <div className="flex items-center gap-2 mb-3">
                  <Info size={14} style={{ color: "var(--yunque-accent)" }} />
                  <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>关系 ({selectedRelations.length})</span>
                </div>
                {selectedRelations.length === 0 ? (
                  <div className="text-xs py-4 text-center" style={{ color: "var(--yunque-text-muted)" }}>无关联关系</div>
                ) : (
                  <div className="space-y-2">
                    {selectedRelations.map(rel => {
                      const isFrom = rel.from_id === selected.id;
                      const otherName = entityName(isFrom ? rel.to_id : rel.from_id);
                      return (
                        <div key={rel.id} className="flex items-center gap-2 p-2 rounded-lg" style={{ background: "rgba(255,255,255,0.02)" }}>
                          <span className="text-xs" style={{ color: "var(--yunque-text)" }}>{selected.name}</span>
                          <span className="text-xs px-1.5 py-0.5 rounded" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>
                            {isFrom ? `→ ${rel.type}` : `← ${rel.type}`}
                          </span>
                          <span className="text-xs" style={{ color: "var(--yunque-text)" }}>{otherName}</span>
                          {rel.weight && (
                            <span className="text-[10px] ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                              {(rel.weight * 100).toFixed(0)}%
                            </span>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </Card>
            </>
          ) : (
            <Card className="section-card p-5">
              <EmptyState
                icon={<Info size={24} style={{ color: "var(--yunque-accent)" }} />}
                title="点击节点查看详情"
                description="在左侧图谱中点击任意节点查看实体详情和关系"
              />
            </Card>
          )}

          {/* Entity list */}
          <Card className="section-card p-5">
            <div className="text-xs font-medium uppercase tracking-wider mb-3" style={{ color: "var(--yunque-text-muted)" }}>
              实体列表 ({entities.length})
            </div>
            <div className="space-y-1 max-h-[300px] overflow-y-auto">
              {entities.map(e => (
                <button
                  key={e.id}
                  onClick={() => handleNodeClick(e)}
                  className={`w-full flex items-center gap-2 p-2 rounded-lg text-left cursor-pointer transition-colors ${selected?.id === e.id ? "" : "hover:bg-white/[0.03]"}`}
                  style={selected?.id === e.id ? { background: "rgba(0,111,238,0.08)" } : undefined}
                >
                  <div className="w-2 h-2 rounded-full shrink-0" style={{ background: typeColor(e.type) }} />
                  <span className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{e.name}</span>
                  <span className="text-[10px] ml-auto shrink-0" style={{ color: "var(--yunque-text-muted)" }}>{e.type}</span>
                </button>
              ))}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
