// Theme snapshot + change observation for the Pack DLC host. The sandboxed
// iframe (opaque origin) cannot read the shell's CSS custom properties, so the
// host collects a curated token set and ships it over the bridge: once in the
// host.handshake response, then as `theme.changed` events on live changes.

/** Curated `--yunque-*` tokens a pack UI needs for visual consistency. */
export const PACK_THEME_TOKENS = [
  "--yunque-bg",
  "--yunque-bg-overlay",
  "--yunque-surface",
  "--yunque-text",
  "--yunque-text-muted",
  "--yunque-border",
  "--yunque-accent",
  "--yunque-accent-hover",
  "--yunque-accent-muted",
  "--yunque-success",
  "--yunque-danger",
  "--yunque-warning",
] as const;

export interface PackTheme {
  mode: "dark" | "light";
  /** Token → resolved value; tokens without a value are omitted. */
  vars: Record<string, string>;
}

/** collectPackTheme snapshots the shell's current theme. Inline styles (set by
 *  theme-engine) win over stylesheet values from getComputedStyle. */
export function collectPackTheme(doc: Document = document): PackTheme {
  const root = doc.documentElement;
  const computed = typeof window !== "undefined" && doc.defaultView
    ? doc.defaultView.getComputedStyle(root)
    : null;
  const vars: Record<string, string> = {};
  for (const token of PACK_THEME_TOKENS) {
    const inline = root.style.getPropertyValue(token).trim();
    const value = inline || (computed?.getPropertyValue(token).trim() ?? "");
    if (value) vars[token] = value;
  }
  return {
    mode: root.classList.contains("light") ? "light" : "dark",
    vars,
  };
}

/** observePackTheme invokes onChange whenever the shell theme changes (class /
 *  data-theme / inline style mutations on <html>). Returns a disposer. */
export function observePackTheme(onChange: (theme: PackTheme) => void, doc: Document = document): () => void {
  if (typeof MutationObserver === "undefined") return () => {};
  let last = JSON.stringify(collectPackTheme(doc));
  const observer = new MutationObserver(() => {
    const next = collectPackTheme(doc);
    const key = JSON.stringify(next);
    if (key !== last) {
      last = key;
      onChange(next);
    }
  });
  observer.observe(doc.documentElement, {
    attributes: true,
    attributeFilter: ["class", "style", "data-theme"],
  });
  return () => observer.disconnect();
}
