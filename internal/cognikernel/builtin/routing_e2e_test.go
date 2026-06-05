package builtin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"yunque-agent/pkg/cogni"
)

// TestCogniSemanticRouting_RealEmbed drives the REAL routing path end-to-end:
// the actual builtin Cogni declarations + a live yunque-embed server + the real
// Registry/Hook/Evaluator + the production floor/weight. It proves that
// paraphrases without any declared keyword still activate the right Cogni, and
// that off-topic messages activate none of the task Cognis.
//
//	$env:YQ_COGNI_E2E="1"; go test ./internal/cognikernel/builtin/ -run RealEmbed -v
func TestCogniSemanticRouting_RealEmbed(t *testing.T) {
	if os.Getenv("YQ_COGNI_E2E") == "" {
		t.Skip("set YQ_COGNI_E2E=1 with the embed server on :8080")
	}
	url := os.Getenv("EMBED_URL")
	if url == "" {
		url = "http://127.0.0.1:8080/v1/embeddings"
	}
	embed := func(text string) []float32 {
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

	decls, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	reg := cogni.NewRegistry()
	for _, d := range decls {
		_ = reg.Add(d, "builtin")
	}
	hook := cogni.NewHook(reg)
	hook.SetEmbedder(embed)

	task := map[string]bool{"office-assistant": true, "data-analyst": true, "creative-assistant": true}

	cases := []struct {
		msg  string
		want string // "" = none of the task Cognis should activate
	}{
		{"把这堆照片弄成一份给领导看的东西", "office-assistant"},
		{"帮我把上个月的成交情况弄成好看的表给老板", "data-analyst"},
		{"看看这些销量背后藏着什么趋势", "data-analyst"},
		{"给新品弄个特别抓眼球的展示画面", "creative-assistant"},
		{"今天天气怎么样", ""},
		{"帮我订一张明天去北京的机票", ""},
	}

	for _, c := range cases {
		acts := hook.Activate(cogni.ContextRequest{Message: c.msg})
		active := map[string][]string{}
		for _, a := range acts {
			if a.Declaration != nil {
				active[a.Declaration.ID] = a.Reasons
			}
		}
		if c.want == "" {
			for id := range task {
				if _, on := active[id]; on {
					t.Errorf("off-topic %q wrongly activated %s (reasons=%v)", c.msg, id, active[id])
				}
			}
			t.Logf("[neg] %q -> no task cogni ✓", c.msg)
			continue
		}
		if _, on := active[c.want]; !on {
			t.Errorf("paraphrase %q did NOT activate %s; active=%v", c.msg, c.want, keysOf(active))
		} else {
			t.Logf("%q -> %s ✓ reasons=%v", c.msg, c.want, active[c.want])
		}
	}
}

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
