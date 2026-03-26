package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ToolResult is the JSON-serializable result returned to the LLM.
type ToolResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// Dispatcher bridges the Engine to the LLM tool calling system.
// Each method corresponds to a named tool: browser_navigate, browser_screenshot, etc.
type Dispatcher struct {
	engine     *Engine
	dataDir    string
	recognizer *Recognizer // optional 4-tier OCR fallback for browser_read
}

func NewDispatcher(engine *Engine, dataDir string) *Dispatcher {
	return &Dispatcher{engine: engine, dataDir: dataDir}
}

// NewDispatcherWithOCR creates a Dispatcher with a 4-tier OCR Recognizer for browser_read fallback.
func NewDispatcherWithOCR(engine *Engine, dataDir string, recognizer *Recognizer) *Dispatcher {
	return &Dispatcher{engine: engine, dataDir: dataDir, recognizer: recognizer}
}

// Dispatch routes a tool call by name.
func (d *Dispatcher) Dispatch(name string, args map[string]any) *ToolResult {
	switch name {
	case "browser_navigate":
		return d.navigate(args)
	case "browser_screenshot":
		return d.screenshot(args)
	case "browser_click":
		return d.click(args)
	case "browser_type":
		return d.typeText(args)
	case "browser_read":
		return d.read(args)
	case "browser_eval":
		return d.eval(args)
	case "browser_close":
		return d.closeBrowser()
	default:
		return &ToolResult{Error: fmt.Sprintf("unknown browser tool: %s", name)}
	}
}

func (d *Dispatcher) navigate(args map[string]any) *ToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return &ToolResult{Error: "url is required"}
	}
	result, err := d.engine.Navigate(url)
	if err != nil {
		return &ToolResult{Error: err.Error()}
	}
	return &ToolResult{OK: true, Data: result}
}

func (d *Dispatcher) screenshot(args map[string]any) *ToolResult {
	name := fmt.Sprintf("screenshot_%d.png", time.Now().UnixMilli())
	path := filepath.Join(d.dataDir, name)

	if err := d.engine.Screenshot(path); err != nil {
		return &ToolResult{Error: err.Error()}
	}
	return &ToolResult{OK: true, Data: map[string]string{"path": path}}
}

func (d *Dispatcher) click(args map[string]any) *ToolResult {
	sel, _ := args["selector"].(string)
	if sel == "" {
		return &ToolResult{Error: "selector is required"}
	}
	if err := d.engine.Click(sel); err != nil {
		return &ToolResult{Error: err.Error()}
	}
	return &ToolResult{OK: true}
}

func (d *Dispatcher) typeText(args map[string]any) *ToolResult {
	sel, _ := args["selector"].(string)
	text, _ := args["text"].(string)
	if sel == "" {
		return &ToolResult{Error: "selector is required"}
	}
	if err := d.engine.Type(sel, text); err != nil {
		return &ToolResult{Error: err.Error()}
	}
	return &ToolResult{OK: true}
}

func (d *Dispatcher) read(args map[string]any) *ToolResult {
	sel, _ := args["selector"].(string)
	hint, _ := args["hint"].(string)

	// Tier 1: DOM extraction (fast, always available)
	text, err := d.engine.ReadText(sel)
	if err == nil && len(strings.TrimSpace(text)) > 20 {
		// DOM has sufficient content
		if len(text) > 8000 {
			text = text[:8000] + "\n... (truncated)"
		}
		return &ToolResult{OK: true, Data: map[string]string{"text": text, "mode": "dom"}}
	}

	// Tier 2-4: OCR fallback chain when DOM text is empty/insufficient
	if d.recognizer != nil {
		result := d.recognizer.ReadTextWithFallback(context.Background(), sel, hint)
		if result.NeedHuman {
			return &ToolResult{OK: false, Error: "all auto OCR tiers failed; human intervention needed",
				Data: map[string]string{"mode": "human_needed"}}
		}
		ocrText := result.Text
		if len(ocrText) > 8000 {
			ocrText = ocrText[:8000] + "\n... (truncated)"
		}
		return &ToolResult{OK: true, Data: map[string]string{
			"text": ocrText, "mode": string(result.Mode),
			"confidence": fmt.Sprintf("%.2f", result.Confidence),
		}}
	}

	// No recognizer configured; return whatever DOM gave us (possibly empty)
	if err != nil {
		return &ToolResult{Error: err.Error()}
	}
	if len(text) > 8000 {
		text = text[:8000] + "\n... (truncated)"
	}
	return &ToolResult{OK: true, Data: map[string]string{"text": text, "mode": "dom"}}
}

func (d *Dispatcher) closeBrowser() *ToolResult {
	d.engine.Close()
	return &ToolResult{OK: true, Data: map[string]string{"status": "browser closed"}}
}

func (d *Dispatcher) eval(args map[string]any) *ToolResult {
	js, _ := args["js"].(string)
	if js == "" {
		return &ToolResult{Error: "js is required"}
	}
	result, err := d.engine.Eval(js)
	if err != nil {
		return &ToolResult{Error: err.Error()}
	}
	return &ToolResult{OK: true, Data: map[string]string{"result": result}}
}

// ToolDefinitions returns LLM function definitions for all browser tools.
func ToolDefinitions() []map[string]any {
	return []map[string]any{
		toolDef("browser_navigate", "打开一个网页URL，返回页面标题和文本预览", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "要打开的URL"},
			},
			"required": []string{"url"},
		}),
		toolDef("browser_screenshot", "对当前页面截图并保存", map[string]any{
			"type": "object", "properties": map[string]any{},
		}),
		toolDef("browser_click", "点击页面上的某个元素", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "CSS选择器"},
			},
			"required": []string{"selector"},
		}),
		toolDef("browser_type", "在输入框中输入文本", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "CSS选择器"},
				"text":     map[string]any{"type": "string", "description": "要输入的文本"},
			},
			"required": []string{"selector", "text"},
		}),
		toolDef("browser_read", "Read page or element text, with automatic OCR fallback when DOM is empty", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "CSS selector, empty to read whole page"},
				"hint":     map[string]any{"type": "string", "description": "Content hint for OCR (e.g. 'captcha text')"},
			},
		}),
		toolDef("browser_eval", "在页面中执行JavaScript代码", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"js": map[string]any{"type": "string", "description": "要执行的JS代码"},
			},
			"required": []string{"js"},
		}),
		toolDef("browser_close", "关闭浏览器。在浏览器任务全部完成后调用此工具释放资源。", map[string]any{
			"type": "object", "properties": map[string]any{},
		}),
	}
}

func toolDef(name, desc string, params map[string]any) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        name,
			"description": desc,
			"parameters":  params,
		},
	}
}

// ensure json import is used
var _ = json.Marshal
