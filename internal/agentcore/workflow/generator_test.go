package workflow

import (
	"context"
	"strings"
	"testing"
	"time"
)

func fixedWorkflowNow() time.Time {
	return time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)
}

func TestGenerateDefinitionFallsBackToTemplateWithoutLLM(t *testing.T) {
	res, err := GenerateDefinition(context.Background(), "每天早上汇总昨天任务并生成日报", GeneratorOptions{
		TenantID: "tenant-a",
		Now:      fixedWorkflowNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != GenerationSourceTemplate {
		t.Fatalf("source=%s, want template", res.Source)
	}
	if res.Definition.TenantID != "tenant-a" {
		t.Fatalf("tenant=%q", res.Definition.TenantID)
	}
	if len(res.Definition.Nodes) < 4 || len(res.Definition.Edges) == 0 {
		t.Fatalf("definition not demo-ready: nodes=%d edges=%d", len(res.Definition.Nodes), len(res.Definition.Edges))
	}
	if res.Definition.Nodes[0].Type != NodeStart || res.Definition.Nodes[len(res.Definition.Nodes)-1].Type != NodeEnd {
		t.Fatalf("template should include start/end nodes: %#v", res.Definition.Nodes)
	}
}

func TestGenerateDefinitionUsesLLMJSON(t *testing.T) {
	raw := "```json\n{" +
		`"name":"日报自动化","description":"demo","nodes":[{"id":"n-1","name":"写日报","type":"llm","config":{"user_prompt":"写日报"},"position":{"x":0,"y":0}}],"edges":[]}` +
		"\n```"
	res, err := GenerateDefinition(context.Background(), "生成日报", GeneratorOptions{
		TenantID: "tenant-b",
		Now:      fixedWorkflowNow,
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			if !strings.Contains(system, "NL2Workflow") {
				t.Fatalf("system prompt should describe NL2Workflow")
			}
			return raw, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != GenerationSourceLLM {
		t.Fatalf("source=%s, want llm", res.Source)
	}
	if res.Definition.ID == "" || res.Definition.Version != 1 {
		t.Fatalf("definition defaults not filled: %#v", res.Definition)
	}
	if !hasNodeType(res.Definition.Nodes, NodeStart) || !hasNodeType(res.Definition.Nodes, NodeEnd) {
		t.Fatalf("normalized LLM workflow should include start/end: %#v", res.Definition.Nodes)
	}
	if res.Definition.Nodes[1].ID != "n_1" {
		t.Fatalf("node id should be sanitized, got %q", res.Definition.Nodes[1].ID)
	}
}

func TestGenerateDefinitionFallsBackToSocialPublishWorkflow(t *testing.T) {
	res, err := GenerateDefinition(context.Background(), "打开小红书创作中心，生成一条效率演示笔记并直接发布", GeneratorOptions{
		TenantID: "tenant-social",
		Now:      fixedWorkflowNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != GenerationSourceTemplate {
		t.Fatalf("source=%s, want template", res.Source)
	}
	if res.Definition.Name != "小红书直发自动化" {
		t.Fatalf("unexpected name: %s", res.Definition.Name)
	}
	if !hasNodeType(res.Definition.Nodes, NodeBrowser) {
		t.Fatalf("social publish workflow should include browser nodes: %#v", res.Definition.Nodes)
	}
	var hasPublish bool
	for _, node := range res.Definition.Nodes {
		if node.ID == "publish" && node.Config["action"] == "click" && node.Config["text_target"] == "发布" {
			hasPublish = true
		}
	}
	if !hasPublish {
		t.Fatalf("social publish workflow should click publish: %#v", res.Definition.Nodes)
	}
}
