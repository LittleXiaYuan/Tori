"use client";

import { useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import type { ChatDispatch } from "@/lib/chat-state";
import { showErrorToast, showToast } from "@/components/toast-provider";

export interface ChatRecordingControls {
  ttsPlaying: string | null;
  isRecording: boolean;
  playTTS: (mId: string, text: string) => Promise<void>;
  startRecording: () => Promise<void>;
  stopRecording: () => void;
}

export function useChatRecording(
  chatD: ChatDispatch,
  inputRef: React.RefObject<HTMLTextAreaElement | null>,
): ChatRecordingControls {
  const [ttsPlaying, setTtsPlaying] = useState<string | null>(null);
  const [isRecording, setIsRecording] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const speechRecRef = useRef<any>(null);

  const playTTS = useCallback(async (mId: string, text: string) => {
    if (ttsPlaying === mId) {
      audioRef.current?.pause();
      setTtsPlaying(null);
      return;
    }
    try {
      setTtsPlaying(mId);
      const buf = await api.tts(text);
      const blob = new Blob([buf], { type: "audio/mp3" });
      const url = URL.createObjectURL(blob);
      if (audioRef.current) audioRef.current.pause();
      const audio = new Audio(url);
      audioRef.current = audio;
      audio.onended = () => { setTtsPlaying(null); URL.revokeObjectURL(url); };
      audio.onerror = () => { setTtsPlaying(null); URL.revokeObjectURL(url); };
      audio.play();
    } catch (e) { setTtsPlaying(null); showErrorToast(e, "语音播放失败，请稍后重试。"); }
  }, [ttsPlaying]);

  const startRecording = useCallback(async () => {
    const SR = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (SR) {
      const rec = new SR();
      rec.lang = "zh-CN";
      rec.interimResults = true;
      rec.continuous = true;
      let finalText = "";
      rec.onresult = (e: { resultIndex: number; results: SpeechRecognitionResultList }) => {
        let interim = "";
        for (let i = e.resultIndex; i < e.results.length; i++) {
          if (e.results[i].isFinal) finalText += e.results[i][0].transcript;
          else interim += e.results[i][0].transcript;
        }
        chatD({ type: "SET_INPUT", value: finalText + interim });
      };
      rec.onerror = () => { setIsRecording(false); };
      rec.onend = () => { setIsRecording(false); inputRef.current?.focus(); };
      rec.start();
      speechRecRef.current = rec;
      setIsRecording(true);
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream, { mimeType: "audio/webm" });
      const chunks: Blob[] = [];
      recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };
      recorder.onstop = async () => {
        stream.getTracks().forEach(t => t.stop());
        const blob = new Blob(chunks, { type: "audio/webm" });
        setIsRecording(false);
        try {
          const result = await api.stt(blob);
          if (result.text) { chatD({ type: "APPEND_INPUT", value: result.text }); inputRef.current?.focus(); }
        } catch { showToast("Speech transcription failed.", "error"); }
      };
      recorder.start();
      mediaRecorderRef.current = recorder;
      setIsRecording(true);
    } catch { showToast("Microphone access failed.", "error"); }
  }, [chatD, inputRef]);

  const stopRecording = useCallback(() => {
    if (speechRecRef.current) { speechRecRef.current.stop(); speechRecRef.current = null; return; }
    mediaRecorderRef.current?.stop();
    mediaRecorderRef.current = null;
  }, []);

  return { ttsPlaying, isRecording, playTTS, startRecording, stopRecording };
}
