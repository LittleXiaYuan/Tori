export type PackReleaseSource = {
  label: string;
  url: string;
  note: string;
};

export const DEFAULT_PACK_RELEASE_SOURCES: PackReleaseSource[] = [
  {
    label: "云雀官方能力包源",
    url: "https://github.com/LittleXiaYuan/Tori/releases/tag/pack%2Fmicro-agent%2Fv0.1.0",
    note: "官方发布的 .yqpack 包，安装前会展示版本、权限和风险。",
  },
];

type SourceInput = string | {
  label?: unknown;
  name?: unknown;
  url?: unknown;
  note?: unknown;
};

export function resolvePackReleaseSources(raw = process.env.NEXT_PUBLIC_YUNQUE_PACK_RELEASE_SOURCES): PackReleaseSource[] {
  const parsed = parsePackReleaseSources(raw);
  return parsed.length > 0 ? parsed : DEFAULT_PACK_RELEASE_SOURCES;
}

export function parsePackReleaseSources(raw?: string): PackReleaseSource[] {
  if (!raw || raw.trim().length === 0) return [];
  const inputs = parseSourceInputs(raw);
  const seen = new Set<string>();
  const sources: PackReleaseSource[] = [];

  for (const input of inputs) {
    const source = normalizeSource(input);
    if (!source || seen.has(source.url)) continue;
    seen.add(source.url);
    sources.push(source);
  }

  return sources;
}

function parseSourceInputs(raw: string): SourceInput[] {
  const trimmed = raw.trim();
  if (trimmed.startsWith("[")) {
    try {
      const parsed = JSON.parse(trimmed);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  return trimmed
    .split(/[\n;,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeSource(input: SourceInput): PackReleaseSource | null {
  if (typeof input === "string") {
    const [labelOrUrl, maybeUrl, maybeNote] = input.split("|").map((part) => part.trim());
    const url = maybeUrl || labelOrUrl;
    if (!isHttpUrl(url)) return null;
    return {
      label: maybeUrl ? labelOrUrl || sourceHost(url) : `能力包源 · ${sourceHost(url)}`,
      url,
      note: maybeNote || "配置的 .yqpack 发布源，安装前会展示版本、权限和风险。",
    };
  }

  const url = typeof input.url === "string" ? input.url.trim() : "";
  if (!isHttpUrl(url)) return null;
  const label = typeof input.label === "string" ? input.label.trim() : typeof input.name === "string" ? input.name.trim() : "";
  const note = typeof input.note === "string" ? input.note.trim() : "";
  return {
    label: label || `能力包源 · ${sourceHost(url)}`,
    url,
    note: note || "配置的 .yqpack 发布源，安装前会展示版本、权限和风险。",
  };
}

function isHttpUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function sourceHost(value: string): string {
  try {
    return new URL(value).host;
  } catch {
    return value;
  }
}
