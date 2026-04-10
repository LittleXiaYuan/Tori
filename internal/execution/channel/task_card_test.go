package channel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskStepCard_Done(t *testing.T) {
	card := TaskStepCard("重构模块", 3, 10, "修改 handler", "done", "ok")
	raw := card.Build()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("invalid card JSON: %v", err)
	}
	header, _ := m["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	content, _ := title["content"].(string)
	if !strings.Contains(content, "3/10") || !strings.Contains(content, "完成") {
		t.Fatalf("unexpected title: %s", content)
	}
}

func TestTaskStepCard_Failed(t *testing.T) {
	card := TaskStepCard("部署", 2, 5, "测试", "failed", "timeout")
	raw := card.Build()
	var m map[string]any
	json.Unmarshal([]byte(raw), &m)
	header, _ := m["header"].(map[string]any)
	tmpl, _ := header["template"].(string)
	if tmpl != "red" {
		t.Fatalf("expected red, got %s", tmpl)
	}
}

func TestTaskCompletedCard(t *testing.T) {
	card := TaskCompletedCard("优化查询", "tsk-1", "时间降低 90%")
	raw := card.Build()
	if !strings.Contains(raw, "任务完成") {
		t.Fatal("should contain 任务完成")
	}
	if !strings.Contains(raw, "tsk-1") {
		t.Fatal("should contain task ID")
	}
}

func TestTaskFailedCard(t *testing.T) {
	card := TaskFailedCard("升级", "tsk-2", "npm error")
	raw := card.Build()
	if !strings.Contains(raw, "任务失败") {
		t.Fatal("should contain 任务失败")
	}
	if !strings.Contains(raw, "retry_task") {
		t.Fatal("should contain retry button")
	}
}

func TestTaskApprovalCard(t *testing.T) {
	card := TaskApprovalCard("删除数据库", "tsk-3", "将删除生产库")
	raw := card.Build()
	if !strings.Contains(raw, "需要审批") {
		t.Fatal("should contain 需要审批")
	}
	if !strings.Contains(raw, "approve") {
		t.Fatal("should contain approve action")
	}
	if !strings.Contains(raw, "deny") {
		t.Fatal("should contain deny action")
	}
}

func TestProgressBar(t *testing.T) {
	bar := progressBar(5, 10)
	if len([]rune(bar)) != 10 {
		t.Fatalf("expected 10 runes, got %d", len([]rune(bar)))
	}
}

// ── Telegram Reply builders ────────────────────────────────

func TestTaskFailedReplyTG(t *testing.T) {
	r := TaskFailedReplyTG("部署任务", "tsk-1", "connection refused")
	if r.Rich == nil {
		t.Fatal("expected RichMessage with buttons")
	}
	buttons := 0
	for _, comp := range r.Rich.Components {
		if comp.Type() == ComponentButton {
			buttons++
		}
	}
	if buttons != 2 {
		t.Fatalf("expected 2 buttons (retry+cancel), got %d", buttons)
	}
	if !strings.Contains(r.Content, "任务失败") {
		t.Fatal("markdown should contain 任务失败")
	}
}

func TestTaskPausedReplyTG(t *testing.T) {
	r := TaskPausedReplyTG("tsk-2")
	if r.Rich == nil {
		t.Fatal("expected RichMessage with resume button")
	}
	btn, ok := r.Rich.Components[0].(*ButtonComponent)
	if !ok {
		t.Fatal("expected ButtonComponent")
	}
	if btn.Value != "resume_task:tsk-2" {
		t.Fatalf("expected resume_task:tsk-2, got %s", btn.Value)
	}
}

func TestTaskApprovalReplyTG(t *testing.T) {
	r := TaskApprovalReplyTG("删除数据", "tsk-3", "将删除生产库")
	if r.Rich == nil || len(r.Rich.Components) != 2 {
		t.Fatal("expected 2 buttons (approve+deny)")
	}
	approve, _ := r.Rich.Components[0].(*ButtonComponent)
	deny, _ := r.Rich.Components[1].(*ButtonComponent)
	if approve.Value != "approve:tsk-3" {
		t.Fatalf("expected approve:tsk-3, got %s", approve.Value)
	}
	if deny.Value != "deny:tsk-3" {
		t.Fatalf("expected deny:tsk-3, got %s", deny.Value)
	}
}
