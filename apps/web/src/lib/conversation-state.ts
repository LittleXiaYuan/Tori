import type { Dispatch } from "react";
import type { ConversationInfo } from "@/lib/api";

export interface ConvState {
  list: ConversationInfo[];
  activeId: string;
  showArchived: boolean;
  searchQuery: string;
  renameId: string | null;
  renameText: string;
}

export type ConvAction =
  | { type: "SET_LIST"; list: ConversationInfo[] }
  | { type: "UPDATE_ONE"; id: string; data: Partial<ConversationInfo> }
  | { type: "REMOVE"; id: string }
  | { type: "SET_ACTIVE"; id: string }
  | { type: "SET_ARCHIVED"; show: boolean }
  | { type: "SET_SEARCH"; query: string }
  | { type: "START_RENAME"; id: string; text: string }
  | { type: "SET_RENAME_TEXT"; text: string }
  | { type: "CANCEL_RENAME" };

export type ConvDispatch = Dispatch<ConvAction>;

export const ACTIVE_CONV_KEY = "yunque_active_conv";

export const convInit: ConvState = {
  list: [],
  activeId:
    (typeof window !== "undefined"
      ? localStorage.getItem(ACTIVE_CONV_KEY)
      : null) || "default",
  showArchived: false,
  searchQuery: "",
  renameId: null,
  renameText: "",
};

export function convReducer(state: ConvState, action: ConvAction): ConvState {
  switch (action.type) {
    case "SET_LIST":
      return { ...state, list: action.list };
    case "UPDATE_ONE":
      return {
        ...state,
        list: state.list.map((c) =>
          c.id === action.id ? { ...c, ...action.data } : c,
        ),
        renameId: null,
      };
    case "REMOVE":
      return {
        ...state,
        list: state.list.filter((c) => c.id !== action.id),
      };
    case "SET_ACTIVE":
      if (typeof window !== "undefined")
        localStorage.setItem(ACTIVE_CONV_KEY, action.id);
      return { ...state, activeId: action.id };
    case "SET_ARCHIVED":
      return { ...state, showArchived: action.show };
    case "SET_SEARCH":
      return { ...state, searchQuery: action.query };
    case "START_RENAME":
      return { ...state, renameId: action.id, renameText: action.text };
    case "SET_RENAME_TEXT":
      return { ...state, renameText: action.text };
    case "CANCEL_RENAME":
      return { ...state, renameId: null, renameText: "" };
    default:
      return state;
  }
}
