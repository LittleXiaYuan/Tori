"use client";

import { useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import type { ChatDispatch } from "@/lib/chat-state";
import { showToast } from "@/components/toast-provider";

export type PendingFile = {
  id: string;
  name: string;
  size: number;
  preview?: string;
  base64?: string;
  workspacePath?: string;
  parsedText?: string;
  parser?: string;
  type: "image" | "video" | "text" | "binary";
  status?: "ready" | "uploading" | "parsed" | "error";
  note?: string;
};

const TEXT_EXTS = new Set([
  "txt","md","csv","json","yaml","yml","toml","xml","html","css","js","ts",
  "tsx","jsx","py","go","rs","rb","java","c","cpp","h","sh","bash","sql",
  "ini","cfg","env","log","gitignore","dockerfile",
]);

function isTextFile(name: string) {
  const ext = name.split(".").pop()?.toLowerCase() || "";
  return TEXT_EXTS.has(ext);
}

export interface ChatMediaControls {
  pendingFiles: PendingFile[];
  setPendingFiles: React.Dispatch<React.SetStateAction<PendingFile[]>>;
  isDragging: boolean;
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  processFile: (file: File) => void;
  handleFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  handleDrop: (e: React.DragEvent) => void;
  handleDragOver: (e: React.DragEvent) => void;
  handleDragLeave: (e: React.DragEvent) => void;
}

export function useChatMedia(
  chatD: ChatDispatch,
  getCurrentInput: () => string,
): ChatMediaControls {
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const processFile = useCallback((file: File) => {
    const fileId = `${file.name}-${file.size}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const isImage = file.type.startsWith("image/");
    const isVideo = file.type.startsWith("video/");
    const isText = isTextFile(file.name) || file.type.startsWith("text/");

    if (isImage || isVideo) {
      const previewUrl = URL.createObjectURL(file);
      const reader = new FileReader();
      reader.onload = () => {
        const base64 = reader.result as string;
        setPendingFiles(prev => [...prev, {
          id: fileId, name: file.name, size: file.size, preview: previewUrl,
          base64, type: isImage ? "image" : "video", status: "ready",
          note: isImage ? "Image ready" : "Video ready",
        }]);
      };
      reader.readAsDataURL(file);
    } else {
      setPendingFiles(prev => [...prev, {
        id: fileId, name: file.name, size: file.size,
        type: isText ? "text" : "binary", status: "uploading",
        note: "正在添加附件…",
      }]);
      api.uploadFile(file).then(res => {
        const parserName = typeof res.parse?.parser === "string" ? res.parse.parser : "";
        const parsePreview = typeof res.parse?.preview === "string" ? res.parse.preview.trim() : "";
        const parseNote = typeof res.parse?.note === "string" ? res.parse.note.trim() : "";
        const parseStatus = typeof res.parse?.status === "string" ? res.parse.status : "";
        const note = parsePreview
          ? "已添加，发送后由模型读取"
          : parseStatus === "needs_document_parser"
            ? (parseNote || "附件已保留，等待文档解析后端展开正文")
            : (parseNote || "附件已添加");
        setPendingFiles(prev => prev.map(item => item.id === fileId ? {
          ...item,
          workspacePath: res.path,
          parsedText: parsePreview || undefined,
          parser: parserName || undefined,
          status: parsePreview ? "parsed" : "ready",
          note,
        } : item));
        if (parsePreview) {
          showToast(`附件 ${file.name} 已添加，发送后由模型读取。`, "success");
        } else if (parseNote) {
          showToast(parseNote, parseStatus === "needs_document_parser" ? "info" : "info");
        }
      }).catch(() => {
        setPendingFiles(prev => prev.map(item => item.id === fileId ? {
          ...item, status: "error", note: "上传失败，尝试作为文本附件加入",
        } : item));
        if (isText) {
          const reader = new FileReader();
          reader.onload = () => {
            const text = String(reader.result || "").slice(0, 12000);
            setPendingFiles(prev => prev.map(item => item.id === fileId ? {
              ...item,
              parsedText: text,
              status: "parsed",
              note: "已添加，发送后由模型读取",
            } : item));
          };
          reader.readAsText(file);
        } else {
          showToast(`文件 ${file.name} 上传失败，请重新上传。`, "error");
        }
      });
    }
  }, [chatD, getCurrentInput]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    Array.from(e.target.files || []).forEach(processFile);
    e.target.value = "";
  }, [processFile]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    Array.from(e.dataTransfer.files).forEach(processFile);
  }, [processFile]);

  const handleDragOver = useCallback((e: React.DragEvent) => { e.preventDefault(); e.stopPropagation(); setIsDragging(true); }, []);
  const handleDragLeave = useCallback((e: React.DragEvent) => { e.preventDefault(); e.stopPropagation(); setIsDragging(false); }, []);

  return {
    pendingFiles, setPendingFiles, isDragging, fileInputRef,
    processFile, handleFileUpload, handleDrop, handleDragOver, handleDragLeave,
  };
}
