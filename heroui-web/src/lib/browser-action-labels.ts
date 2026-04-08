export function browserActionLabel(action?: string | null) {
  switch (action) {
    case "bridge/switch-to-tab":
      return "??????";
    case "bridge/takeover":
      return "?????";
    case "bridge/resume":
      return "??????";
    case "/navigate":
      return "????";
    case "/screenshot":
      return "????";
    case "/content":
      return "??????";
    case "/mark":
      return "??????";
    case "/unmark":
      return "??????";
    case "/scroll":
      return "????";
    case "/click":
      return "????";
    case "/type":
      return "????";
    default:
      return (action || "?????").replace("bridge/", "");
  }
}
