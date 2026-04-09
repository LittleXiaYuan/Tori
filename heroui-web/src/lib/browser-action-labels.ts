const ACTION_LABELS: Record<string, string> = {
  "bridge/switch-to-tab": "Return to tab",
  "bridge/takeover": "Hand control to you",
  "bridge/resume": "Resume browser run",
  "/navigate": "Open page",
  "/screenshot": "Capture page",
  "/content": "Read page",
  "/mark": "Mark elements",
  "/unmark": "Clear markers",
  "/scroll": "Scroll page",
  "/click": "Click element",
  "/type": "Type input",
  "browser_navigate": "Open page",
  "browser_screenshot": "Capture page",
  "browser_get_content": "Read page",
  "browser_mark_elements": "Mark elements",
  "browser_unmark_elements": "Clear markers",
  "browser_scroll": "Scroll page",
  "browser_click": "Click element",
  "browser_input": "Type input",
  "browser_switch_tab": "Switch tab",
  "browser_list_tabs": "List tabs",
  "browser_new_tab": "Open new tab",
  "browser_close_tab": "Close tab",
  "browser_takeover": "Hand control to you",
};

export function browserActionLabel(action?: string | null) {
  if (!action) return "Browser action";
  return ACTION_LABELS[action] || action.replace("bridge/", "").replace("browser_", "").replaceAll("_", " ");
}

export function browserActionPhase(action?: string | null, stage: "start" | "success" | "error" | "handoff" = "success") {
  const label = browserActionLabel(action);
  switch (stage) {
    case "start":
      return `${label} in progress`;
    case "error":
      return `${label} failed`;
    case "handoff":
      return `${label}; waiting for you`;
    default:
      return `${label} completed`;
  }
}
