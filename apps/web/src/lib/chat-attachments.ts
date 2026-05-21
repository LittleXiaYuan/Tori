import type { PendingFile } from "@/lib/use-chat-media";

export type ChatContextAttachment = {
  name: string;
  mime: string;
  dataB64: string;
};

export function utf8ToBase64(input: string): string {
  const bytes = new TextEncoder().encode(input);
  let binary = "";
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.slice(i, i + chunkSize));
  }
  return btoa(binary);
}

export function buildHiddenContextAttachments(files: PendingFile[]): ChatContextAttachment[] {
  return files.flatMap((f) => {
    if (f.type === "image" || f.type === "video") return [];
    if (f.parsedText) {
      return [{
        name: f.name,
        mime: "text/plain; charset=utf-8",
        dataB64: utf8ToBase64([
          `[Parsed document: ${f.name}]`,
          `Workspace path: ${f.workspacePath || f.name}`,
          `Parser: ${f.parser || "local"}`,
          `Status: ${f.status || "parsed"}`,
          f.note ? `Note: ${f.note}` : "",
          "Instruction: This parsed document body is hidden working context. Do not echo the full body into chat unless the user explicitly asks to display the complete extracted text; summarize or use only the relevant fields by default.",
          "",
          f.parsedText || "",
        ].join("\n")),
      }];
    }
    if (f.workspacePath || f.note) {
      return [{
        name: f.name,
        mime: "text/x-yunque-attachment-metadata; charset=utf-8",
        dataB64: utf8ToBase64([
          `[Attachment file: ${f.name}]`,
          `Workspace path: ${f.workspacePath || f.name}`,
          `Parser: ${f.parser || "unknown"}`,
          `Status: ${f.status || "ready"}`,
          f.note ? `Note: ${f.note}` : "",
          "Instruction: The full document body is not available in this attachment metadata. Do not claim you have read the full document; use this metadata only to decide the next parsing or follow-up step.",
        ].filter(Boolean).join("\n")),
      }];
    }
    return [];
  });
}
