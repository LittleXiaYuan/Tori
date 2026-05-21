import { describe, expect, it } from "vitest";

import { buildHiddenContextAttachments } from "../chat-attachments";
import type { PendingFile } from "../use-chat-media";

function decodeUtf8B64(dataB64: string): string {
  const binary = atob(dataB64);
  const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function parsedFile(name: string, parsedText: string): PendingFile {
  return {
    id: name,
    name,
    size: parsedText.length,
    type: "binary",
    status: "parsed",
    workspacePath: `uploads/${name}`,
    parsedText,
  };
}

describe("buildHiddenContextAttachments", () => {
  it("keeps DOCX/XLSX/PPTX parsed text as hidden attachments", () => {
    const attachments = buildHiddenContextAttachments([
      parsedFile("申请表.docx", "公司名称\t云鸢科技"),
      parsedFile("预算.xlsx", "[Sheet: 预算]\n项目\t金额"),
      parsedFile("路演.pptx", "[Slide 1]\n云雀 Agent"),
    ]);

    expect(attachments.map((a) => a.name)).toEqual(["申请表.docx", "预算.xlsx", "路演.pptx"]);
    expect(attachments.every((a) => a.mime === "text/plain; charset=utf-8")).toBe(true);
    expect(decodeUtf8B64(attachments[0].dataB64)).toContain("[Parsed document: 申请表.docx]");
    expect(decodeUtf8B64(attachments[0].dataB64)).toContain("Parser: local");
    expect(decodeUtf8B64(attachments[0].dataB64)).toContain("Status: parsed");
    expect(decodeUtf8B64(attachments[0].dataB64)).toContain("hidden working context");
    expect(decodeUtf8B64(attachments[0].dataB64)).toContain("Do not echo the full body into chat");
    expect(decodeUtf8B64(attachments[1].dataB64)).toContain("[Sheet: 预算]");
    expect(decodeUtf8B64(attachments[2].dataB64)).toContain("[Slide 1]");
  });

  it("sends unparsed document metadata without pretending it is parsed text", () => {
    const attachments = buildHiddenContextAttachments([{
      id: "pdf",
      name: "申请表.pdf",
      size: 1024,
      type: "binary",
      status: "ready",
      workspacePath: "uploads/申请表.pdf",
      note: "附件已添加，但当前本地解析器还不能直接展开 .pdf 正文；配置文档解析后端后会自动提取正文。",
    }]);

    expect(attachments).toHaveLength(1);
    expect(attachments[0].mime).toBe("text/x-yunque-attachment-metadata; charset=utf-8");
    const decoded = decodeUtf8B64(attachments[0].dataB64);
    expect(decoded).toContain("[Attachment file: 申请表.pdf]");
    expect(decoded).toContain("Workspace path: uploads/申请表.pdf");
    expect(decoded).toContain("Parser: unknown");
    expect(decoded).toContain("Status: ready");
    expect(decoded).toContain("正文；配置文档解析后端后会自动提取正文");
    expect(decoded).toContain("The full document body is not available");
    expect(decoded).toContain("Do not claim you have read the full document");
    expect(decoded).not.toContain("[Parsed document:");
  });

  it("does not duplicate image or video uploads as text metadata attachments", () => {
    const attachments = buildHiddenContextAttachments([
      {
        id: "img",
        name: "截图.png",
        size: 2048,
        type: "image",
        status: "ready",
        base64: "data:image/png;base64,AAAA",
        note: "Image ready",
      },
      {
        id: "video",
        name: "演示.mp4",
        size: 4096,
        type: "video",
        status: "ready",
        base64: "data:video/mp4;base64,AAAA",
        note: "Video ready",
      },
    ]);

    expect(attachments).toEqual([]);
  });
});
