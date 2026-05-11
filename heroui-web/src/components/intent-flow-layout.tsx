"use client";

import { createContext, useContext, useReducer, useCallback, type ReactNode, type Dispatch } from "react";

/**
 * IntentFlowLayout — Agent-driven adaptive layout system.
 *
 * The Agent backend signals the user's detected intent (e.g. "browsing",
 * "coding", "researching", "chatting") and the layout automatically
 * adjusts which panels are visible, their sizes, and emphasis.
 *
 * Layout zones:
 *  ┌────────┬──────────────────────┬───────────┐
 *  │  Rail  │     Main Content     │  Aux Panel│
 *  │(fixed) │  (flexible center)   │ (adaptive)│
 *  │        │                      │           │
 *  └────────┴──────────────────────┴───────────┘
 *       ↑         ↑                      ↑
 *    always    chat / editor         context-dependent:
 *    visible   / dashboard          computer, trace,
 *                                   evolution, browser
 */

export type UserIntent =
  | "chatting"
  | "coding"
  | "browsing"
  | "researching"
  | "reviewing"
  | "idle";

export type AuxPanelType =
  | "none"
  | "computer"
  | "trace"
  | "evolution"
  | "browser"
  | "skill_growth"
  | "cognitive";

export interface LayoutState {
  intent: UserIntent;
  auxPanel: AuxPanelType;
  auxWidth: number;
  sidebarVisible: boolean;
  zenMode: boolean;
  emphasis: "main" | "aux" | "balanced";
  previousIntent: UserIntent | null;
  transitionMs: number;
}

type LayoutAction =
  | { type: "SET_INTENT"; intent: UserIntent }
  | { type: "SET_AUX_PANEL"; panel: AuxPanelType }
  | { type: "SET_AUX_WIDTH"; width: number }
  | { type: "TOGGLE_SIDEBAR" }
  | { type: "SET_ZEN"; zen: boolean }
  | { type: "SET_EMPHASIS"; emphasis: LayoutState["emphasis"] }
  | { type: "RESET" };

const intentDefaults: Record<UserIntent, Partial<LayoutState>> = {
  chatting:     { auxPanel: "none",     auxWidth: 0,   emphasis: "main",     sidebarVisible: true },
  coding:       { auxPanel: "computer", auxWidth: 420, emphasis: "balanced", sidebarVisible: false },
  browsing:     { auxPanel: "browser",  auxWidth: 480, emphasis: "aux",      sidebarVisible: false },
  researching:  { auxPanel: "trace",    auxWidth: 360, emphasis: "balanced", sidebarVisible: true },
  reviewing:    { auxPanel: "cognitive",auxWidth: 320, emphasis: "main",     sidebarVisible: true },
  idle:         { auxPanel: "none",     auxWidth: 0,   emphasis: "main",     sidebarVisible: true },
};

const defaultState: LayoutState = {
  intent: "chatting",
  auxPanel: "none",
  auxWidth: 0,
  sidebarVisible: true,
  zenMode: false,
  emphasis: "main",
  previousIntent: null,
  transitionMs: 300,
};

function layoutReducer(state: LayoutState, action: LayoutAction): LayoutState {
  switch (action.type) {
    case "SET_INTENT": {
      const defaults = intentDefaults[action.intent] || {};
      return {
        ...state,
        ...defaults,
        intent: action.intent,
        previousIntent: state.intent,
        transitionMs: 300,
      };
    }
    case "SET_AUX_PANEL":
      return {
        ...state,
        auxPanel: action.panel,
        auxWidth: action.panel === "none" ? 0 : (state.auxWidth || 340),
      };
    case "SET_AUX_WIDTH":
      return { ...state, auxWidth: Math.max(0, Math.min(action.width, 600)) };
    case "TOGGLE_SIDEBAR":
      return { ...state, sidebarVisible: !state.sidebarVisible };
    case "SET_ZEN":
      return {
        ...state,
        zenMode: action.zen,
        sidebarVisible: action.zen ? false : state.sidebarVisible,
        auxPanel: action.zen ? "none" : state.auxPanel,
        auxWidth: action.zen ? 0 : state.auxWidth,
      };
    case "SET_EMPHASIS":
      return { ...state, emphasis: action.emphasis };
    case "RESET":
      return defaultState;
    default:
      return state;
  }
}

interface LayoutContextValue {
  state: LayoutState;
  dispatch: Dispatch<LayoutAction>;
  setIntent: (intent: UserIntent) => void;
  openAux: (panel: AuxPanelType) => void;
  closeAux: () => void;
  toggleSidebar: () => void;
}

const LayoutContext = createContext<LayoutContextValue | null>(null);

export function useIntentFlowLayout(): LayoutContextValue {
  const ctx = useContext(LayoutContext);
  if (!ctx) throw new Error("useIntentFlowLayout must be used within IntentFlowProvider");
  return ctx;
}

export function IntentFlowProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(layoutReducer, defaultState);

  const setIntent = useCallback((intent: UserIntent) => {
    dispatch({ type: "SET_INTENT", intent });
  }, []);

  const openAux = useCallback((panel: AuxPanelType) => {
    dispatch({ type: "SET_AUX_PANEL", panel });
  }, []);

  const closeAux = useCallback(() => {
    dispatch({ type: "SET_AUX_PANEL", panel: "none" });
  }, []);

  const toggleSidebar = useCallback(() => {
    dispatch({ type: "TOGGLE_SIDEBAR" });
  }, []);

  return (
    <LayoutContext.Provider value={{ state, dispatch, setIntent, openAux, closeAux, toggleSidebar }}>
      {children}
    </LayoutContext.Provider>
  );
}

interface IntentFlowShellProps {
  rail?: ReactNode;
  sidebar?: ReactNode;
  main: ReactNode;
  aux?: ReactNode;
}

export function IntentFlowShell({ rail, sidebar, main, aux }: IntentFlowShellProps) {
  const { state } = useIntentFlowLayout();

  const mainFlex = state.emphasis === "aux" ? "0.6" : state.emphasis === "balanced" ? "1" : "1";

  return (
    <div
      className="flex h-full w-full overflow-hidden"
      style={{ transition: `all ${state.transitionMs}ms var(--ease-out)` }}
    >
      {/* Rail (always visible unless zen) */}
      {!state.zenMode && rail && (
        <div className="shrink-0" style={{ width: "var(--rail-w)" }}>
          {rail}
        </div>
      )}

      {/* Sidebar (conditional) */}
      {!state.zenMode && state.sidebarVisible && sidebar && (
        <div
          className="shrink-0 overflow-y-auto border-r"
          style={{
            width: "var(--conv-rail-w)",
            borderColor: "var(--yunque-border)",
            transition: `width ${state.transitionMs}ms var(--ease-out)`,
          }}
        >
          {sidebar}
        </div>
      )}

      {/* Main content */}
      <div
        className="flex-1 min-w-0 overflow-y-auto"
        style={{ flex: mainFlex }}
      >
        {main}
      </div>

      {/* Aux panel (intent-driven) */}
      {state.auxPanel !== "none" && aux && (
        <div
          className="shrink-0 overflow-y-auto border-l"
          style={{
            width: state.auxWidth,
            borderColor: "var(--yunque-border)",
            transition: `width ${state.transitionMs}ms var(--ease-out)`,
          }}
        >
          {aux}
        </div>
      )}
    </div>
  );
}

export function intentToLabel(intent: UserIntent): string {
  const labels: Record<UserIntent, string> = {
    chatting: "对话模式",
    coding: "编码模式",
    browsing: "浏览模式",
    researching: "研究模式",
    reviewing: "审阅模式",
    idle: "待机",
  };
  return labels[intent] || intent;
}

export default IntentFlowProvider;
