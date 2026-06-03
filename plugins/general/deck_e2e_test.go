package general

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/pkg/skills"
)

// TestDeckE2E drives the real production path: env.LLMCall -> DeepSeek (same
// model the agent uses) -> JSON spec -> Go design system -> headless Chrome PDF.
// Skipped unless YQ_DECK_E2E=1 and DEEPSEEK_KEY are set.
//   $env:YQ_DECK_E2E="1"; $env:DEEPSEEK_KEY="..."; go test ./plugins/general/ -run TestDeckE2E -v -timeout 300s
func TestDeckE2E(t *testing.T) {
	if os.Getenv("YQ_DECK_E2E") == "" {
		t.Skip("set YQ_DECK_E2E=1 to run the live DeepSeek end-to-end deck test")
	}
	key := os.Getenv("DEEPSEEK_KEY")
	base := os.Getenv("DEEPSEEK_BASE")
	model := os.Getenv("DEEPSEEK_MODEL")
	if key == "" {
		t.Fatal("DEEPSEEK_KEY required")
	}
	if base == "" {
		base = "https://api.deepseek.com/v1"
	}
	if model == "" {
		model = "deepseek-v4-pro"
	}

	llm := func(ctx context.Context, system, user string) (string, error) {
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
			"temperature": 0.6,
			"max_tokens":  4000,
			"stream":      false,
		})
		req, _ := http.NewRequestWithContext(ctx, "POST", base+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := (&http.Client{Timeout: 200 * time.Second}).Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", err
		}
		if out.Error != nil {
			t.Fatalf("deepseek error: %s", out.Error.Message)
		}
		if len(out.Choices) == 0 {
			t.Fatalf("deepseek empty: %s", string(raw))
		}
		return out.Choices[0].Message.Content, nil
	}

	outDir := deckTestDir()
	uploads := filepath.Join(outDir, "uploads")
	_ = os.MkdirAll(outDir, 0755)
	env := &skills.Environment{LLMCall: llm}
	skill := NewDeckCreateSkill([]string{outDir, uploads}, []string{outDir})

	ctx, cancel := context.WithTimeout(context.Background(), 280*time.Second)
	defer cancel()

	brief := `云雀 Yunque 是一个"真正记得你"的 AI 陪伴助手。核心卖点:
1) 长期语义记忆——跨会话记住用户偏好/背景,换个说法也能精准召回(真机评测 20/20 命中);
2) 认知内核 Cogni——会规划、会反思、会主动学习,按场景切换性情与能力;
3) 原生创作——一句话生成 PPT/Word/表格,设计对标 Gamma/Kimi,用户零依赖;
4) 隐私安全——多租户严格隔离,本地优先;
5) 多模型智能路由——成本与性能最优。
受众:潜在用户与投资人。目标:让人相信"它真的懂你、还能替你做事"。`

	t0 := time.Now()
	args := map[string]any{
		"path":   filepath.Join(outDir, "e2e_aurora.pdf"),
		"brief":  brief,
		"title":  "云雀 Yunque",
		"style":  "aurora",
		"slides": 10,
	}
	if _, e := os.Stat(uploads); e == nil {
		args["images"] = uploads // let DeepSeek place these into image layouts
	}
	msg, err := skill.Execute(ctx, args, env)
	if err != nil {
		t.Fatalf("Execute failed after %v: %v", time.Since(t0), err)
	}
	t.Logf("E2E ok in %v: %s", time.Since(t0), msg)
}
