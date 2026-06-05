package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/skills"
)

// fakeSkill is a no-op skill used only to exercise the tool-surface filter.
type fakeSkill struct{ name string }

func (s fakeSkill) Name() string                  { return s.name }
func (s fakeSkill) Description() string            { return s.name }
func (s fakeSkill) Parameters() map[string]any     { return map[string]any{} }
func (s fakeSkill) Execute(context.Context, map[string]any, *skills.Environment) (string, error) {
	return "", nil
}

func realEmbedderForTest() cogni.EmbedderFunc {
	url := os.Getenv("EMBED_URL")
	if url == "" {
		url = "http://127.0.0.1:8080/v1/embeddings"
	}
	return func(text string) []float32 {
		body, _ := json.Marshal(map[string]any{"input": []string{text}, "model": "yunque-embed-v1"})
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &out) != nil || len(out.Data) == 0 {
			return nil
		}
		return out.Data[0].Embedding
	}
}

// TestCogniReceipts_RealEmbed proves Cogni is LIVE (not paper) by printing, for
// real user messages with the real builtin Cognis + real embedder, the three
// concrete effects: (1) which Cogni activates, (2) the exact text injected into
// the system prompt, (3) the tool surface narrowing (before -> after).
//
//	$env:YQ_COGNI_E2E="1"; go test ./internal/cognikernel/builtin/ -run Receipts -v
func TestCogniReceipts_RealEmbed(t *testing.T) {
	if os.Getenv("YQ_COGNI_E2E") == "" {
		t.Skip("set YQ_COGNI_E2E=1 with the embed server on :8080")
	}
	decls, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	reg := cogni.NewRegistry()
	for _, d := range decls {
		_ = reg.Add(d, "builtin")
	}
	hook := cogni.NewHook(reg)
	hook.SetEmbedder(realEmbedderForTest())

	// A realistic full tool set (~38) like the agent exposes by default.
	names := []string{
		"docx_create", "docx_fill", "docx_edit", "xlsx_create", "xlsx_fill", "xlsx_edit",
		"xlsx_split", "pptx_create", "pptx_fill", "pptx_edit", "pptx_template_search",
		"deck_create", "pdf_create", "html_export", "image_gen", "file_write", "file_search",
		"file_open", "zip_unpack", "zip_pack", "web_search", "translate", "doc_parse",
		"code_gen", "browser", "send_email", "computer_use", "shell_exec", "workflow_gen",
		"http_request", "sql_query", "calendar", "contacts", "notes", "reminder",
		"weather", "maps", "calculator",
	}
	all := make([]skills.Skill, len(names))
	for i, n := range names {
		all[i] = fakeSkill{name: n}
	}

	msgs := []string{
		"帮我做一份漂亮的PPT用来路演",         // explicit keyword
		"把这堆照片弄成一份给领导看的东西",     // paraphrase, no keyword
		"今天天气怎么样",                   // off-topic control
	}

	for _, msg := range msgs {
		req := cogni.ContextRequest{Message: msg}
		acts := hook.Activate(req)
		ctxBlock := hook.BuildContext(req)
		filtered := hook.FilterSkills(req, all)

		ids := []string{}
		for _, a := range acts {
			if a.Declaration != nil {
				ids = append(ids, a.Declaration.ID)
			}
		}
		t.Logf("\n================ MSG: %s ================", msg)
		t.Logf("① 激活的 Cogni: %v", ids)
		for _, a := range acts {
			if a.Declaration != nil {
				t.Logf("   - %s score=%.2f reasons=%v", a.Declaration.ID, a.Score, a.Reasons)
			}
		}
		t.Logf("② 注入系统提示词 (%d 字节):\n%s", len([]rune(ctxBlock)), ctxBlock)
		t.Logf("③ 工具集裁剪: %d -> %d (裁掉 %d 个)", len(all), len(filtered), len(all)-len(filtered))
	}
}
