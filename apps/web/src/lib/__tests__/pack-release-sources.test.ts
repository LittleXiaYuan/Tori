import { describe, expect, it } from "vitest";
import { DEFAULT_PACK_RELEASE_SOURCES, parsePackReleaseSources, resolvePackReleaseSources } from "../pack-release-sources";

describe("pack release sources", () => {
  it("falls back to the official source when config is empty", () => {
    expect(resolvePackReleaseSources("")).toEqual(DEFAULT_PACK_RELEASE_SOURCES);
  });

  it("parses multiple URL sources from a simple config string", () => {
    expect(parsePackReleaseSources([
      "官方源|https://github.com/LittleXiaYuan/Tori/releases/tag/pack%2Fmicro-agent%2Fv0.1.0|GitHub Release",
      "https://oss.example.com/yunque/packs/catalog.json",
    ].join("\n"))).toEqual([
      {
        label: "官方源",
        url: "https://github.com/LittleXiaYuan/Tori/releases/tag/pack%2Fmicro-agent%2Fv0.1.0",
        note: "GitHub Release",
      },
      {
        label: "能力包源 · oss.example.com",
        url: "https://oss.example.com/yunque/packs/catalog.json",
        note: "配置的 .yqpack 发布源，安装前会展示版本、权限和风险。",
      },
    ]);
  });

  it("parses JSON source objects and drops invalid or duplicated URLs", () => {
    const raw = JSON.stringify([
      { label: "OSS 源", url: "https://oss.example.com/packs/release", note: "团队 OSS" },
      { url: "https://oss.example.com/packs/release" },
      { label: "坏源", url: "file:///tmp/pack" },
    ]);

    expect(parsePackReleaseSources(raw)).toEqual([
      {
        label: "OSS 源",
        url: "https://oss.example.com/packs/release",
        note: "团队 OSS",
      },
    ]);
  });
});
