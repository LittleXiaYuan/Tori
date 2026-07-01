const PROVIDER_ID_HINT = /\bprovider(?:[_\s-]?id)?\s*[:=]\s*["']?([a-z0-9][a-z0-9_.:-]{1,80})/i;
const PROVIDER_TOKEN = /[a-z0-9][a-z0-9_.:-]{2,80}/gi;
const PROVIDER_FAMILIES = [
  "openai",
  "qwen",
  "moonshot",
  "kimi",
  "deepseek",
  "minimax",
  "gemini",
  "google",
  "anthropic",
  "claude",
  "ollama",
  "tori",
  "local",
];

function isSpecificProviderId(token: string): boolean {
  const value = token.trim().toLowerCase();
  if (!value || value === "provider" || value === "model" || value === "llm") return false;
  if (!/[-_:]/.test(value)) return false;
  return PROVIDER_FAMILIES.some((family) => value.includes(family));
}

export function providerIdFromText(parts: Array<string | null | undefined>): string {
  for (const part of parts) {
    if (!part) continue;
    const explicit = part.match(PROVIDER_ID_HINT)?.[1];
    if (explicit && isSpecificProviderId(explicit)) return explicit.toLowerCase();
  }

  const text = parts.filter(Boolean).join(" ");
  for (const match of text.matchAll(PROVIDER_TOKEN)) {
    const token = match[0];
    if (isSpecificProviderId(token)) return token.toLowerCase();
  }
  return "";
}

export function providerFocusHrefFromText(parts: Array<string | null | undefined>): string {
  const providerId = providerIdFromText(parts);
  return providerId ? `/settings/providers?focus=${encodeURIComponent(providerId)}` : "/settings/providers?tab=providers";
}
