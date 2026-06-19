import { describe, expect, it } from "vitest";
import {
  packStudioWorkspaceMatches,
  parsePackStudioBatchDraftRequestPrompt,
  parsePackStudioPatchDraftRequestPrompt,
  parsePackStudioPatchDraftPrompt,
  parsePackStudioPatchPlanPrompt,
} from "../pack-studio-chat";

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

describe("parsePackStudioPatchDraftPrompt", () => {
  it("extracts a single-file draft that still needs Studio diff preview", () => {
    const parsed = parsePackStudioPatchDraftPrompt([
      "小羽给出的单文件草稿：",
      "```json",
      JSON.stringify({
        kind: "yunque.pack_studio.patch_draft.v1",
        pack: { id: "yunque.pack.wasm-plugin", name: "WASM 能力包", version: "0.1.0" },
        goal: "增加结果界面",
        workspace: {
          id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          original_sha256: "a".repeat(64),
        },
        file_path: "C:\\yunque\\packs\\studio\\pack.json",
        content: "{\n  \"description\": \"更清楚\"\n}\n",
        reason: "补强说明",
        risk_level: "low",
        gates: ["预览 diff", "内置审计"],
      }, null, 2),
      "```",
    ].join("\n"));

    expect(parsed?.pack.id).toBe("yunque.pack.wasm-plugin");
    expect(parsed?.filePath).toBe("C:\\yunque\\packs\\studio\\pack.json");
    expect(parsed?.content).toContain("\"description\": \"更清楚\"");
    expect(parsed?.gates).toEqual(["预览 diff", "内置审计"]);
  });

  it("requires a real file path and content", () => {
    expect(parsePackStudioPatchDraftPrompt("```json\n{\"kind\":\"yunque.pack_studio.patch_draft.v1\"}\n```")).toBeNull();
  });
});

describe("parsePackStudioPatchDraftRequestPrompt", () => {
  it("extracts a draft generation request without exposing starter content as display text", () => {
    const starterContent = "<!doctype html>\n<p>完整草稿内容不应该直接展示</p>\n";
    const parsed = parsePackStudioPatchDraftRequestPrompt([
      "请让小羽生成单文件 Draft。",
      "",
      "下面是 Pack Studio 的 Patch Draft Request。",
      "```json",
      JSON.stringify({
        kind: "yunque.pack_studio.patch_draft_request.v1",
        pack: { id: "yunque.pack.wasm-plugin", name: "WASM 能力包", version: "0.1.0" },
        goal: "增加结果界面",
        workspace: {
          id: "yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          path: "C:\\yunque\\packs\\studio\\yunque.pack.wasm-plugin-0.1.0-aaaaaaaaaaaa",
          original_sha256: "a".repeat(64),
        },
        target: {
          file_path: "C:\\yunque\\packs\\studio\\frontend\\index.html",
          label: "前端界面草稿",
          reason: "补结果界面",
          risk_level: "medium",
          gates: ["预览 diff", "内置审计"],
          content_summary: { length: starterContent.length, hash: "feedbeef" },
        },
        starter_content: starterContent,
        expected_output: { kind: "yunque.pack_studio.patch_draft.v1" },
      }, null, 2),
      "```",
    ].join("\n"));

    expect(parsed?.pack.id).toBe("yunque.pack.wasm-plugin");
    expect(parsed?.target.filePath).toBe("C:\\yunque\\packs\\studio\\frontend\\index.html");
    expect(parsed?.target.contentSummary).toEqual({ length: starterContent.length, hash: "feedbeef" });
    expect(parsed?.starterContentLength).toBe(starterContent.length);
    expect(parsed?.expectedKind).toBe("yunque.pack_studio.patch_draft.v1");
    expect(parsed?.displayText).toBe("请让小羽生成单文件 Draft。");
    expect(parsed?.displayText).not.toContain("完整草稿内容不应该直接展示");
  });

  it("requires a target file path", () => {
    expect(parsePackStudioPatchDraftRequestPrompt("```json\n{\"kind\":\"yunque.pack_studio.patch_draft_request.v1\"}\n```")).toBeNull();
  });
});

describe("parsePackStudioBatchDraftRequestPrompt", () => {
  it("extracts a batch draft request and hides the structured JSON from display text", () => {
    const parsed = parsePackStudioBatchDraftRequestPrompt([
      "请批量补肉这些能力包。",
      "",
      "```json",
      JSON.stringify({
        kind: "yunque.pack_studio.batch_draft_request.v1",
        goal: "把看得到但不知道怎么用的能力包补成可打开、可验证、可回滚。",
        batch: { page: 2, page_count: 4, total: 22, page_size: 6 },
        rules: ["不要自动应用改动", "逐包生成 Draft Request"],
        packs: [
          {
            id: "yunque.pack.needs-entry",
            name: "Needs Entry Pack",
            version: "0.1.0",
            status: "beta",
            source: "已安装",
            missing: ["使用示例", "打开/使用入口"],
            readiness: "需补入口",
            delivery: {
              level: "needs_meat",
              label: "待补肉",
              description: "用户装上后容易不知道怎么验证。",
              next_step: "交给小羽先补入口。",
            },
            studio_url: "/packs/studio?pack=yunque.pack.needs-entry",
            package_url: "https://example.com/yunque.pack.needs-entry.yqpack",
            sha256: "a".repeat(64),
          },
        ],
      }, null, 2),
      "```",
    ].join("\n"));

    expect(parsed?.goal).toContain("可打开");
    expect(parsed?.batch).toEqual({ page: 2, pageCount: 4, total: 22, pageSize: 6 });
    expect(parsed?.rules).toEqual(["不要自动应用改动", "逐包生成 Draft Request"]);
    expect(parsed?.packs[0]).toMatchObject({
      id: "yunque.pack.needs-entry",
      name: "Needs Entry Pack",
      readiness: "需补入口",
      studioUrl: "/packs/studio?pack=yunque.pack.needs-entry",
      packageUrl: "https://example.com/yunque.pack.needs-entry.yqpack",
      delivery: {
        level: "needs_meat",
        label: "待补肉",
        description: "用户装上后容易不知道怎么验证。",
        nextStep: "交给小羽先补入口。",
      },
    });
    expect(parsed?.packs[0].missing).toEqual(["使用示例", "打开/使用入口"]);
    expect(parsed?.displayText).toBe("请批量补肉这些能力包。");
    expect(parsed?.displayText).not.toContain("yunque.pack_studio.batch_draft_request.v1");
  });

  it("requires the batch request kind and a packs array", () => {
    expect(parsePackStudioBatchDraftRequestPrompt("```json\n{\"kind\":\"other\",\"packs\":[]}\n```")).toBeNull();
    expect(parsePackStudioBatchDraftRequestPrompt("```json\n{\"kind\":\"yunque.pack_studio.batch_draft_request.v1\"}\n```")).toBeNull();
  });
});

describe("packStudioWorkspaceMatches", () => {
  const imported = {
    id: "ws-id",
    path: "C:\\studio\\pack",
    originalSha256: "a".repeat(64),
  };

  it("matches by id, path or original sha", () => {
    expect(packStudioWorkspaceMatches(imported, { workspace_id: "ws-id" })).toBe(true);
    expect(packStudioWorkspaceMatches(imported, { workspace_path: "c:/studio/pack" })).toBe(true);
    expect(packStudioWorkspaceMatches(imported, { original_sha256: "a".repeat(64) })).toBe(true);
  });

  it("rejects unrelated workspaces", () => {
    expect(packStudioWorkspaceMatches(imported, {
      workspace_id: "other",
      workspace_path: "C:\\other",
      original_sha256: "b".repeat(64),
    })).toBe(false);
  });
});
