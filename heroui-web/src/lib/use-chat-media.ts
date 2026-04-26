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
        note: "Uploading to workspace...",
      }]);
      const input = getCurrentInput();
      api.uploadFile(file).then(res => {
        const parsePreview = typeof res.parse?.preview === "string" ? res.parse.preview.trim() : "";
        const uploadLine = parsePreview
          ? [`[Parsed document: ${file.name}]`, `Workspace path: ${res.path}`, "", parsePreview].join("\n")
          : `Uploaded file: ${res.path}`;
        chatD({ type: "SET_INPUT", value: input + (input ? "\n" : "") + uploadLine });
        setPendingFiles(prev => prev.map(item => item.id === fileId ? {
          ...item,
          status: res.parse?.parser === "mineru" ? "parsed" : "ready",
          note: res.parse?.parser === "mineru" ? "Parsed by MinerU" : `Saved to ${res.path}`,
        } : item));
        if (res.parse?.parser === "mineru") {
          showToast(`Parsed ${file.name} with MinerU.`, "success");
        }
      }).catch(() => {
        setPendingFiles(prev => prev.map(item => item.id === fileId ? {
          ...item, status: "error", note: "Upload failed, using local fallback",
        } : item));
        if (isText) {
          const reader = new FileReader();
          reader.onload = () => {
            const text = reader.result as string;
            chatD({ type: "SET_INPUT", value: input + (input ? "\n" : "") + `[File: ${file.name}]\n${text.slice(0, 4000)}` });
          };
          reader.readAsText(file);
        } else {
          const sizeStr = file.size > 1024 * 1024 ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : `${(file.size / 1024).toFixed(1)} KB`;
          chatD({ type: "SET_INPUT", value: input + (input ? "\n" : "") + `[File: ${file.name} (${sizeStr})]` });
        }
      });
    }
  }, [chatD, getCurrentInput]);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) processFile(file);
    e.target.value = "";
  }, [processFile]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    Array.from(e.dataTransfer.files).forEach(processFile);
  }, [processFile]);

  const handleDragOver = useCallback((e: React.DragEvent) => { e.preventDefault(); setIsDragging(true); }, []);
  const handleDragLeave = useCallback((e: React.DragEvent) => { e.preventDefault(); setIsDragging(false); }, []);

  return {
    pendingFiles, setPendingFiles, isDragging, fileInputRef,
    processFile, handleFileUpload, handleDrop, handleDragOver, handleDragLeave,
  };
}
