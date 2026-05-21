import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CherrySettingsModal } from "../settings-modal";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

afterEach(() => {
  vi.restoreAllMocks();
  push.mockReset();
});

function mockJsonSequence(payloads: unknown[]) {
  const queue = [...payloads];
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const path = String(input);
    if (path.includes("/v1/backup/export") || path.includes("/v1/backup/import")) {
      throw new Error(`unexpected direct backup action: ${path}`);
    }
    const payload = queue.shift() ?? {};
    return new Response(JSON.stringify(payload), { status: 200 });
  });
}

describe("Cherry settings Pack Runtime entrypoints", () => {
  it("links enabled backup pack to /packs/backup without executing backup actions", async () => {
    const fetchSpy = mockJsonSequence([
      { data_dir: "C:/yunque/data", db_size_mb: 1 },
      {
        packs: [
          {
            status: "enabled",
            manifest: { id: "yunque.pack.backup", name: "Backup Pack", version: "0.1.0" },
          },
        ],
        count: 1,
      },
    ]);

    render(<CherrySettingsModal open onClose={() => {}} initialSection="data" />);

    expect(await screen.findByText("当前状态：已启用")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "打开备份 Pack" }));

    expect(push).toHaveBeenCalledWith("/packs/backup");
    expect(fetchSpy.mock.calls.map((call) => String(call[0]))).toContain("/v1/packs/installed");
    expect(fetchSpy.mock.calls.map((call) => String(call[0]))).not.toContain("/v1/backup/export");
  });

  it("routes missing backup pack to the Pack console for installation", async () => {
    mockJsonSequence([
      { data_dir: "C:/yunque/data", db_size_mb: 1 },
      { packs: [], count: 0 },
    ]);

    render(<CherrySettingsModal open onClose={() => {}} initialSection="data" />);

    await waitFor(() => expect(screen.getByText("当前状态：未安装")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "安装 Pack" }));

    expect(push).toHaveBeenCalledWith("/packs");
  });
});
