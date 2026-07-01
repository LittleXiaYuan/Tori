const CONNECTOR_MATCHERS = [
  { terms: ["github"], id: "github" },
  { terms: ["gmail"], id: "gmail" },
  { terms: ["google_calendar", "google calendar"], id: "google_calendar" },
  { terms: ["slack"], id: "slack" },
  { terms: ["notion"], id: "notion" },
  { terms: ["linear"], id: "linear" },
  { terms: ["jira"], id: "jira" },
];

export function connectorIdFromText(parts: Array<string | null | undefined>): string {
  const text = parts.filter(Boolean).join(" ").toLowerCase();
  return CONNECTOR_MATCHERS.find((item) => item.terms.some((term) => text.includes(term)))?.id || "";
}

export function connectorFocusHrefFromText(parts: Array<string | null | undefined>): string {
  const connectorId = connectorIdFromText(parts);
  return connectorId ? `/settings/connectors?focus=${encodeURIComponent(connectorId)}` : "/settings/connectors";
}
