import type { Dispatch } from "react";
import type { AgentEvent } from "@/components/execution-trace";
import type { Message } from "@/lib/chat-types";

export interface ChatState {
  messages: Message[];
  input: string;
  loading: boolean;
  streaming: boolean;
  liveTraceEvents: AgentEvent[];
}

export type ChatAction =
  | { type: "SET_INPUT"; value: string }
  | { type: "APPEND_INPUT"; value: string }
  | { type: "SET_MESSAGES"; messages: Message[] }
  | { type: "ADD_PAIR"; userMsg: Message; asstMsg: Message }
  | { type: "APPEND_LAST"; delta: string }
  | { type: "UPDATE_LAST"; updates: Partial<Message> }
  | { type: "APPEND_LAST_TRACE"; event: AgentEvent }
  | { type: "APPEND_LAST_REASONING"; delta: string }
  | { type: "ERROR_LAST"; error: string }
  | { type: "REMOVE_MSG"; id: string }
  | { type: "START_SEND" }
  | { type: "FINISH_SEND" }
  | { type: "ADD_LIVE_TRACE"; event: AgentEvent }
  | { type: "CLEAR_LIVE_TRACE" };

export type ChatDispatch = Dispatch<ChatAction>;

export const chatInit: ChatState = {
  messages: [],
  input: "",
  loading: false,
  streaming: false,
  liveTraceEvents: [],
};

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case "SET_INPUT":
      return { ...state, input: action.value };
    case "APPEND_INPUT":
      return { ...state, input: state.input + action.value };
    case "SET_MESSAGES":
      return { ...state, messages: action.messages };
    case "ADD_PAIR":
      return {
        ...state,
        messages: [...state.messages, action.userMsg, action.asstMsg],
      };
    case "APPEND_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = msgs[msgs.length - 1];
      msgs[msgs.length - 1] = { ...last, content: last.content + action.delta };
      return { ...state, messages: msgs };
    }
    case "UPDATE_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      msgs[msgs.length - 1] = { ...msgs[msgs.length - 1], ...action.updates };
      return { ...state, messages: msgs };
    }
    case "APPEND_LAST_TRACE": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = { ...msgs[msgs.length - 1] };
      last.traceEvents = [...(last.traceEvents || []), action.event];
      msgs[msgs.length - 1] = last;
      const liveTraceEvents = [
        ...state.liveTraceEvents.slice(-50),
        action.event,
      ];
      return { ...state, messages: msgs, liveTraceEvents };
    }
    case "APPEND_LAST_REASONING": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = { ...msgs[msgs.length - 1] };
      if (!last.reasoning) last.reasoningStartMs = Date.now();
      last.reasoning = (last.reasoning || "") + action.delta;
      msgs[msgs.length - 1] = last;
      return { ...state, messages: msgs };
    }
    case "ERROR_LAST": {
      const msgs = [...state.messages];
      if (msgs.length === 0) return state;
      const last = msgs[msgs.length - 1];
      msgs[msgs.length - 1] = {
        ...last,
        content: last.content + `\n\n[FAIL] ${action.error}`,
      };
      return { ...state, messages: msgs };
    }
    case "REMOVE_MSG":
      return {
        ...state,
        messages: state.messages.filter((m) => m.id !== action.id),
      };
    case "START_SEND":
      return {
        ...state,
        input: "",
        loading: true,
        streaming: true,
        liveTraceEvents: [],
      };
    case "FINISH_SEND": {
      const finMsgs = [...state.messages];
      if (finMsgs.length > 0) {
        const last = { ...finMsgs[finMsgs.length - 1] };
        if (last.reasoning && last.reasoningStartMs && !last.reasoningEndMs) {
          last.reasoningEndMs = Date.now();
          finMsgs[finMsgs.length - 1] = last;
        }
      }
      return {
        ...state,
        messages: finMsgs,
        loading: false,
        streaming: false,
      };
    }
    case "ADD_LIVE_TRACE":
      return {
        ...state,
        liveTraceEvents: [...state.liveTraceEvents.slice(-50), action.event],
      };
    case "CLEAR_LIVE_TRACE":
      return { ...state, liveTraceEvents: [] };
    default:
      return state;
  }
}
