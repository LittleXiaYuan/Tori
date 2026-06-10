import { afterEach, describe, expect, it, vi } from "vitest";
import { collectPackTheme, observePackTheme } from "../pack-theme";

afterEach(() => {
  const root = document.documentElement;
  root.removeAttribute("style");
  root.classList.remove("dark", "light");
});

describe("pack-theme/collectPackTheme", () => {
  it("snapshots inline --yunque-* tokens and the mode class", () => {
    const root = document.documentElement;
    root.classList.add("light");
    root.style.setProperty("--yunque-accent", "#006fee");
    root.style.setProperty("--yunque-text", "rgb(28,32,48)");
    root.style.setProperty("--unrelated", "ignored");

    const theme = collectPackTheme();
    expect(theme.mode).toBe("light");
    expect(theme.vars["--yunque-accent"]).toBe("#006fee");
    expect(theme.vars["--yunque-text"]).toBe("rgb(28,32,48)");
    expect(theme.vars).not.toHaveProperty("--unrelated");
  });

  it("defaults to dark mode when no light class is present", () => {
    expect(collectPackTheme().mode).toBe("dark");
  });
});

describe("pack-theme/observePackTheme", () => {
  it("fires on theme mutations and stops after dispose", async () => {
    const onChange = vi.fn();
    const stop = observePackTheme(onChange);

    document.documentElement.classList.add("light");
    await vi.waitFor(() => expect(onChange).toHaveBeenCalledTimes(1));
    expect(onChange.mock.calls[0][0].mode).toBe("light");

    stop();
    document.documentElement.style.setProperty("--yunque-accent", "#ff0000");
    await new Promise((r) => setTimeout(r, 30));
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("does not fire for mutations that leave the snapshot unchanged", async () => {
    const onChange = vi.fn();
    const stop = observePackTheme(onChange);
    // data-foo is not observed; style mutation of an unrelated var changes the
    // style attribute but not the collected snapshot.
    document.documentElement.style.setProperty("--unrelated", "x");
    await new Promise((r) => setTimeout(r, 30));
    expect(onChange).not.toHaveBeenCalled();
    stop();
  });
});
