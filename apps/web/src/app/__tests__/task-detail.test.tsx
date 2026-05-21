import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ExecutionPanel, TaskThreadMessage } from "../task-detail/page";
import type { TaskInfo } from "@/lib/api-types";

describe("Task detail execution panel", () => {
  it("shows preserved checkpoint evidence naturally without raw diagnostics", () => {
    const task: TaskInfo = {
      id: "task-resume-evidence",
      title: "恢复：根据申请表生成材料",
      description: "恢复 Planner 后台任务",
      status: "running",
      tenant_id: "tenant-a",
      created_at: "2026-05-11T00:00:00Z",
      updated_at: "2026-05-11T00:00:00Z",
      steps: [
        {
          id: 1,
          action: "根据附件生成申请材料",
          skill_name: "doc_writer",
          status: "pending",
          input: [
            "[读取附件]: [Parsed document: 申请表.docx]",
            "Workspace path: C:\\Code\\AI\\云雀\\yunque-agent\\tmp\\申请表.docx",
            "公司名称\t云鸢科技",
            '工具返回：handoff agent "file_exec" execution failed: context deadline exceeded EOF',
          ].join("\n"),
          result: '阶段结果：公司名称 云鸢科技\nhandoff agent "general_exec" execution failed: EOF',
          metadata: { planner_step_id: 1 },
        },
      ],
    };

    render(<ExecutionPanel task={task} />);

    expect(screen.getByText("接续输入（已保留证据）")).toBeInTheDocument();
    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getAllByText(/公司名称\s+云鸢科技/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/任务暂时没有完成，已保留现场/).length).toBeGreaterThan(0);
    expect(screen.getByText("原规划步骤 #1")).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace path:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();
  });
});

describe("Task detail thread message", () => {
  it("keeps thread evidence readable while hiding raw parser and runtime diagnostics", () => {
    render(<TaskThreadMessage msg={{
      role: "assistant",
      content: [
        "已接收附件：[Parsed document: 申请表.docx]",
        "Workspace path: C:\\Code\\AI\\云雀\\yunque-agent\\tmp\\申请表.docx",
        "公司名称\t云鸢科技",
        'handoff agent "file_exec" execution failed: context deadline exceeded EOF',
      ].join("\n"),
    }} />);

    expect(screen.getByText(/附件内容：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/附件名称：申请表\.docx/)).toBeInTheDocument();
    expect(screen.getByText(/公司名称\s+云鸢科技/)).toBeInTheDocument();
    expect(screen.getByText(/响应暂时超时，已保留现场/)).toBeInTheDocument();
    expect(screen.queryByText(/\[Parsed document:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace path:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/handoff agent/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/execution failed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\bEOF\b/)).not.toBeInTheDocument();
  });
});
