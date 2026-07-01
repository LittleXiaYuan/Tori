import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ConnectorsPage from "../settings/connectors/page";

const routerMock = vi.hoisted(() => ({
  push: vi.fn(),
}));

const searchParamsMock = vi.hoisted(() => ({
  value: new URLSearchParams(),
}));

const apiMock = vi.hoisted(() => ({
  connectorList: vi.fn(),
  connectorConnect: vi.fn(),
  connectorDisconnect: vi.fn(),
}));

const browserClientMock = vi.hoisted(() => ({
  extensionStatus: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
  useSearchParams: () => searchParamsMock.value,
}));

vi.mock("@/lib/api", () => ({
  api: apiMock,
}));

vi.mock("@/lib/browser-intent-pack-client", () => ({
  createBrowserIntentPackClient: () => browserClientMock,
}));

const githubConnector = {
  id: "github",
  name: "GitHub",
  description: "Access repositories, issues, pull requests, and code on GitHub.",
  icon: "github",
  category: "developer",
  auth_type: "token",
  supported: true,
  status: "connected",
  user_info: "octo",
  action_count: 7,
  allowlist_count: 7,
  allowed_actions: ["list_repos", "get_repo", "list_issues", "create_issue", "list_prs", "search_code", "get_file"],
  last_event: {
    kind: "execute",
    connector_id: "github",
    action_id: "list_repos",
    status: "ok",
    at: "2026-06-24T00:00:00Z",
  },
};

const gmailConnector = {
  id: "gmail",
  name: "Gmail",
  description: "Read, send, and manage your Gmail messages.",
  icon: "mail",
  category: "communication",
  auth_type: "oauth2",
  beta: true,
  supported: true,
  status: "disconnected",
  action_count: 4,
  allowlist_count: 4,
  allowed_actions: ["list_messages", "get_message", "send_message", "list_labels"],
};

const jiraConnector = {
  id: "jira",
  name: "Jira",
  description: "Manage issues and projects in Atlassian Jira.",
  icon: "jira",
  category: "developer",
  auth_type: "token",
  beta: true,
  supported: false,
  status: "disconnected",
  action_count: 2,
  allowlist_count: 0,
};

function mockConnectors(overrides?: { browserConnected?: boolean; connectors?: unknown[] }) {
  apiMock.connectorList.mockResolvedValue({
    connectors: overrides?.connectors ?? [githubConnector, gmailConnector, jiraConnector],
  });
  browserClientMock.extensionStatus.mockResolvedValue({
    connected: overrides?.browserConnected ?? true,
  });
}

describe("ConnectorsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParamsMock.value = new URLSearchParams();
    mockConnectors();
  });

  it("summarizes connector safety before the app list", async () => {
    render(<ConnectorsPage />);

    const overview = (await screen.findByRole("heading", { name: "连接器安全状态" })).closest(".section-card");
    expect(overview).not.toBeNull();

    const section = within(overview as HTMLElement);
    expect(section.getByText("已配对")).toBeInTheDocument();
    expect(section.getByText("1/2")).toBeInTheDocument();
    expect(section.getByText("0 项")).toBeInTheDocument();
    expect(section.getByText("Allowlist")).toBeInTheDocument();
    expect(section.getByText("11 个动作")).toBeInTheDocument();
    expect(section.queryByText("真实浏览器可点击/输入/提取。")).not.toBeInTheDocument();
    expect(section.queryByText("已授权应用可被任务调用。")).not.toBeInTheDocument();
    expect(section.queryByText("未发现连接错误。")).not.toBeInTheDocument();
    expect(section.queryByText("7 个已连接动作。")).not.toBeInTheDocument();
    expect(section.queryByText("浏览器、应用和动作面可用。")).not.toBeInTheDocument();
    expect(section.queryByText("先修浏览器、异常连接或授权。")).not.toBeInTheDocument();
    expect(section.queryByText(/第三方应用和动作面都处在可用状态/)).not.toBeInTheDocument();
    expect(section.queryByText(/可在真实浏览器里点击、输入和提取页面/)).not.toBeInTheDocument();
    expect(section.getByRole("button", { name: "刷新连接器安全状态" })).toBeInTheDocument();
    expect(await screen.findByText("选择一个连接器")).toBeInTheDocument();
    expect(screen.getByText("选应用后配置授权。")).toBeInTheDocument();
    expect(screen.queryByText("能力边界")).not.toBeInTheDocument();
    expect(screen.queryByText("allowlist 动作：4")).not.toBeInTheDocument();
    expect(screen.queryByText("连接后直接说：列 GitHub 仓库、看今日日程。")).not.toBeInTheDocument();
    expect(screen.queryByText(/可以直接在聊天中使用自然语言调用/)).not.toBeInTheDocument();
    expect(screen.queryByText("最近事件")).not.toBeInTheDocument();
    expect(screen.queryByText("暂无事件")).not.toBeInTheDocument();
  });

  it("opens connector details only after an app is selected", async () => {
    render(<ConnectorsPage />);

    const gmailButtons = await screen.findAllByRole("button", { name: /Gmail/ });
    fireEvent.click(gmailButtons[0]);

    expect(screen.getByText("能力边界")).toBeInTheDocument();
    expect(screen.getByText("allowlist 动作：4")).toBeInTheDocument();
    expect(screen.getByLabelText("Access Token")).toBeInTheDocument();
    expect(screen.queryByText("最近事件")).not.toBeInTheDocument();
    expect(screen.queryByText("暂无事件")).not.toBeInTheDocument();
    expect(screen.queryByText("连接后直接说：列 GitHub 仓库、看今日日程。")).not.toBeInTheDocument();
  });

  it("offers a direct browser-pack recovery action when the browser is not paired", async () => {
    mockConnectors({ browserConnected: false, connectors: [gmailConnector, jiraConnector] });

    render(<ConnectorsPage />);

    const overview = (await screen.findByRole("heading", { name: "连接器安全状态" })).closest(".section-card");
    expect(overview).not.toBeNull();

    const section = within(overview as HTMLElement);
    expect(section.getByText("未配对")).toBeInTheDocument();
    expect(section.getByText("0/1")).toBeInTheDocument();
    expect(section.getByText("浏览器未在线，网页任务暂停。")).toBeInTheDocument();
    expect(section.getByText("未连接应用，跨工具流不可用。")).toBeInTheDocument();
    expect(section.getByText("先修浏览器、异常连接或授权。")).toBeInTheDocument();
    expect(section.getByText("先处理红黄项，再进入单个应用配置。")).toBeInTheDocument();
    expect(section.queryByText(/网页执行任务会被暂停/)).not.toBeInTheDocument();

    fireEvent.click(section.getAllByRole("button", { name: "打开浏览器包" })[0]);
    expect(routerMock.push).toHaveBeenCalledWith("/packs/browser");
  });

  it("shows a compact recovery hint for expired connector credentials", async () => {
    mockConnectors({
      connectors: [{
        ...gmailConnector,
        status: "error",
        error: "oauth token expired",
        last_event: {
          kind: "refresh",
          connector_id: "gmail",
          status: "error",
          message: "oauth token expired",
          at: "2026-06-24T00:00:00Z",
        },
      }],
    });

    render(<ConnectorsPage />);

    expect(await screen.findByText("Gmail 凭据需要重新授权")).toBeInTheDocument();
    expect(screen.getByText("OAuth 或 Token 已失效。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重新授权" }));
    expect(screen.getByLabelText("Access Token")).toHaveFocus();
  });

  it("focuses a connector from the recovery link query", async () => {
    searchParamsMock.value = new URLSearchParams("focus=github");

    render(<ConnectorsPage />);

    expect(await screen.findByText("能力边界")).toBeInTheDocument();
    expect(screen.getByText("allowlist 动作：7")).toBeInTheDocument();
    expect(screen.getByText("执行 list_repos成功")).toBeInTheDocument();
  });

  it("keeps unsupported connector guidance compact", async () => {
    render(<ConnectorsPage />);

    const jiraButtons = await screen.findAllByRole("button", { name: /Jira/ });
    fireEvent.click(jiraButtons[0]);

    expect(screen.getByText("后端尚未接入")).toBeInTheDocument();
    expect(screen.getByText("无服务端 handler；coming soon，不能连接。")).toBeInTheDocument();
    expect(screen.queryByText(/暂未接入服务端 handler/)).not.toBeInTheDocument();
    expect(screen.queryByText(/这个连接器的卡片已经展示在产品里/)).not.toBeInTheDocument();
  });
});
