"use client";

import { useState } from "react";
import { Chip, Tabs } from "@heroui/react";
import { Globe } from "lucide-react";
import PageHeader from "@/components/page-header";
import { PackAbout, type PackBoundaryItem } from "@/components/packs/pack-page-kit";
import InnerLifePackPage from "../inner-life/page";
import WorldModelPackPage from "../world-model/page";
import ExperiencePackPage from "../experience/page";
import CognitiveCanaryPackPage from "../cognitive-canary/page";

const boundaryItems: PackBoundaryItem[] = [
  { key: "rules", label: "不写生产规则", detail: "各探测器以本地观测和计划生成为主，不会自动写入生产规则。", tone: "warning" },
  { key: "actions", label: "不触发外部动作", detail: "这是表现层聚合，不会自动触发外部动作。", tone: "warning" },
];

// Inner World is a presentation-layer consolidation of four self-contained
// introspection packs (inner-life, world-model, experience, cognitive-canary)
// into one tabbed surface. Each tab renders the original pack page unchanged —
// the backend modules and their routes are untouched, so this stays fully
// reversible. React Aria Tabs unmounts inactive panels, so each sub-page's data
// fetch only fires when its tab is opened.
export default function InnerWorldPackPage() {
  const [tab, setTab] = useState("inner-life");
  return (
    <div className="space-y-4">
      <PageHeader icon={<Globe size={20} />} title="内在世界" />
      <PackAbout
        chips={<>
          <Chip size="sm" variant="soft">表现层聚合</Chip>
          <Chip size="sm" variant="soft">四类自省探测器</Chip>
        </>}
        description="把四类自省与认知探测器聚到一个面板：查看 Agent 的内在生活与反思、浏览它对外部世界的认知与因果推理、回看沉淀下来的经验、运行认知金丝雀对照实验，无需逐个打开独立页面。"
        boundaries={boundaryItems}
      />
      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="内在世界">
            <Tabs.Tab id="inner-life">内在生活<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="world-model"><Tabs.Separator />世界模型<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="experience"><Tabs.Separator />经验<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="cognitive-canary"><Tabs.Separator />认知金丝雀<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="inner-life">
          <div className="mt-2"><InnerLifePackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="world-model">
          <div className="mt-2"><WorldModelPackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="experience">
          <div className="mt-2"><ExperiencePackPage /></div>
        </Tabs.Panel>
        <Tabs.Panel id="cognitive-canary">
          <div className="mt-2"><CognitiveCanaryPackPage /></div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
