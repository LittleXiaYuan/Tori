import { describe, expect, it } from "vitest";
import { parsePackStudioPatchPlanPrompt } from "../pack-studio-chat";

function promptWithPlan() {
  return [
    "请以小羽改包方式优化能力包。",
    "",
    "下面是 Pack Studio 已准备好的 Patch Plan。请只把它当作结构化导航和安全约束。",
    "",
    "```json",
    JSON.stringify({
      kind: "yunque.pack_studio.patch_plan.v1",
      pack: { id: "yunque.pack.wasm-plugin", name: "WASM 能力包", version: "0.1.0" },
      goal: "增加结果界面",
      workspace: {
        id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
        original_sha256: "a".repeat(64),
      },
      candidates: [
        {
          key: "manifest:C:\\yunque\\packs\\studio\\pack.json",
          label: "manifest 草稿",
          file_path: "C:\\yunque\\packs\\studio\\pack.json",
          risk_level: "low",
          applyable: true,
          gates: ["预览 diff", "内置审计"],
          content_summary: { length: 1200, hash: "abcd1234" },
        },
      ],
    }, null, 2),
    "```",
  ].join("\n");
}

describe("parsePackStudioPatchPlanPrompt", () => {
  it("extracts a structured patch plan from a chat handoff prompt", () => {
    const parsed = parsePackStudioPatchPlanPrompt(promptWithPlan());

    expect(parsed?.pack.id).toBe("yunque.pack.wasm-plugin");
    expect(parsed?.workspace.id).toBe("yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa");
    expect(parsed?.candidates[0]).toMatchObject({
      label: "manifest 草稿",
      filePath: "C:\\yunque\\packs\\studio\\pack.json",
      riskLevel: "low",
      applyable: true,
    });
    expect(parsed?.candidates[0].contentSummary).toEqual({ length: 1200, hash: "abcd1234" });
    expect(parsed?.displayText).toBe("请以小羽改包方式优化能力包。");
  });

  it("ignores invalid or unrelated JSON blocks", () => {
    expect(parsePackStudioPatchPlanPrompt("```json\n{\"kind\":\"other\"}\n```")).toBeNull();
    expect(parsePackStudioPatchPlanPrompt("yunque.pack_studio.patch_plan.v1\n```json\nnot-json\n```")).toBeNull();
  });
});
