"use client";

import { useState } from "react";
import { Chip, Tabs } from "@heroui/react";
import { ShieldAlert } from "lucide-react";
import PageHeader from "@/components/page-header";
import { PackAbout, type PackBoundaryItem } from "@/components/packs/pack-page-kit";
import GuardrailFuzzerPackPage from "../guardrail-fuzzer/page";
import SkillAnomalyPackPage from "../skill-anomaly/page";
import SBOMDriftPackPage from "../sbom-drift/page";
import ChaosProbePackPage from "../chaos-probe/page";

const boundaryItems: PackBoundaryItem[] = [
  { key: "rules", label: "不写生产规则", detail: "各探测器以本地探针和计划生成为主，不会自动写入生产规则。", tone: "warning" },
  { key: "actions", label: "不触发外部动作", detail: "这是表现层聚合，不会自动触发外部动作。", tone: "warning" },
];

// Security Lab is a presentation-layer consolidation of four self-contained
// security/ops packs (guardrail-fuzzer, skill-anomaly, sbom-drift, chaos-probe)
// into one tabbed surface. Each tab renders the original pack page unchanged —
// the backend modules and their routes are untouched, so this stays fully
// reversible. React Aria Tabs unmounts inactive panels, so each sub-page's data
// fetch only fires when its tab is opened.
export default function SecurityLabPackPage() {
  const [tab, setTab] = useState("guardrail-fuzzer");
  return (
    <div className="space-y-4">
      <PageHeader icon={<ShieldAlert size={20} />} title="安全实验室" />
      <PackAbout
        chips={<>
          <Chip size="sm" color="warning">实验中 (alpha)</Chip>
          <Chip size="sm" variant="soft">表现层聚合</Chip>
          <Chip size="sm" variant="soft">四类安全探测器</Chip>
        </>}
        description="把四类安全与韧性探测器聚到一个面板：运行护栏 fuzz、查看 Skill 行为异常、扫描 SBOM 依赖漂移、执行混沌探针，无需逐个打开独立页面。"
        boundaries={boundaryItems}
      />
      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="安全实验室">
            <Tabs.Tab id="guardrail-fuzzer">护栏 Fuzz<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="skill-anomaly"><Tabs.Separator />Skill 异常<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="sbom-drift"><Tabs.Separator />SBOM 漂移<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="chaos-probe"><Tabs.Separator />混沌探针<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="guardrail-fuzzer">
          <div className="mt-2"><GuardrailFuzzerPackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="skill-anomaly">
          <div className="mt-2"><SkillAnomalyPackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="sbom-drift">
          <div className="mt-2"><SBOMDriftPackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="chaos-probe">
          <div className="mt-2"><ChaosProbePackPage /></div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
