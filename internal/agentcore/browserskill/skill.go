package browserskill

import (
	"context"
	"encoding/json"
	"fmt"

	"yunque-agent/pkg/skills"
)

// BrowserController is the interface for sending actions to the browser extension.
type BrowserController interface {
	Connected() bool
	SendAction(ctx context.Context, action any) (any, error)
}

// RegisterSkills registers all browser-related skills into the registry.
func RegisterSkills(reg *skills.Registry, ctrl BrowserController) {
	reg.Register(&navigateSkill{ctrl: ctrl})
	reg.Register(&clickSkill{ctrl: ctrl})
	reg.Register(&inputSkill{ctrl: ctrl})
	reg.Register(&screenshotSkill{ctrl: ctrl})
	reg.Register(&scrollSkill{ctrl: ctrl})
	reg.Register(&getContentSkill{ctrl: ctrl})
	reg.Register(&pressKeySkill{ctrl: ctrl})
	reg.Register(&markElementsSkill{ctrl: ctrl})
	reg.Register(&unmarkElementsSkill{ctrl: ctrl})
	reg.Register(&getElementsSkill{ctrl: ctrl})
	reg.Register(&listTabsSkill{ctrl: ctrl})
	reg.Register(&switchTabSkill{ctrl: ctrl})
	reg.Register(&newTabSkill{ctrl: ctrl})
	reg.Register(&closeTabSkill{ctrl: ctrl})
	reg.Register(&takeoverSkill{ctrl: ctrl})
}

// ─── Navigate ────────────────────────────────────────

type navigateSkill struct{ ctrl BrowserController }

func (s *navigateSkill) Name() string        { return "browser_navigate" }
func (s *navigateSkill) Description() string  { return "Navigate the browser to a URL. After navigation, use browser_mark_elements to find interactive elements, then browser_input/browser_click to interact." }
func (s *navigateSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{{Name: "url", Type: "string", Desc: "The URL to navigate to", Required: true}})
}
func (s *navigateSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}
	result, err := callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_navigate", "url": url})
	if err != nil {
		return result, err
	}
	return result + "\n[Hint: Page loaded. Use browser_mark_elements to see interactive elements, then browser_input/browser_click to interact with the page.]", nil
}

// ─── Click ───────────────────────────────────────────

type clickSkill struct{ ctrl BrowserController }

func (s *clickSkill) Name() string        { return "browser_click" }
func (s *clickSkill) Description() string  { return "Click on a page element by CSS selector, index, or coordinates." }
func (s *clickSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{
		{Name: "selector", Type: "string", Desc: "CSS selector"},
		{Name: "index", Type: "number", Desc: "Interactive element index"},
		{Name: "coordinate_x", Type: "number", Desc: "X coordinate"},
		{Name: "coordinate_y", Type: "number", Desc: "Y coordinate"},
	})
}
func (s *clickSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	action := map[string]any{"type": "browser_click"}
	target := map[string]any{}
	if sel, ok := args["selector"].(string); ok && sel != "" {
		target["strategy"] = "bySelector"
		target["selector"] = sel
	} else if idx, ok := toFloat(args["index"]); ok {
		target["strategy"] = "byIndex"
		target["index"] = int(idx)
	} else if x, xok := toFloat(args["coordinate_x"]); xok {
		y, _ := toFloat(args["coordinate_y"])
		target["strategy"] = "byCoordinates"
		target["coordinateX"] = x
		target["coordinateY"] = y
	} else {
		return "", fmt.Errorf("one of selector, index, or coordinates is required")
	}
	action["target"] = target
	return callBrowser(ctx, s.ctrl, action)
}

// ─── Input ───────────────────────────────────────────

type inputSkill struct{ ctrl BrowserController }

func (s *inputSkill) Name() string        { return "browser_input" }
func (s *inputSkill) Description() string  { return "Type text into a page element. Optionally press Enter after typing." }
func (s *inputSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{
		{Name: "text", Type: "string", Desc: "Text to type", Required: true},
		{Name: "selector", Type: "string", Desc: "CSS selector for the input field"},
		{Name: "press_enter", Type: "boolean", Desc: "Press Enter after typing"},
	})
}
func (s *inputSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	text, _ := args["text"].(string)
	action := map[string]any{"type": "browser_input", "text": text}
	if sel, ok := args["selector"].(string); ok && sel != "" {
		action["target"] = map[string]any{"strategy": "bySelector", "selector": sel}
	}
	if pe, ok := args["press_enter"].(bool); ok {
		action["press_enter"] = pe
	}
	return callBrowser(ctx, s.ctrl, action)
}

// ─── Screenshot ──────────────────────────────────────

type screenshotSkill struct{ ctrl BrowserController }

func (s *screenshotSkill) Name() string        { return "browser_screenshot" }
func (s *screenshotSkill) Description() string  { return "View the current browser page. Returns page title, URL, and interactive elements (use element index for browser_click)." }
func (s *screenshotSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *screenshotSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	screenshotResult, err := callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_screenshot"})
	if err != nil {
		return screenshotResult, err
	}
	contentResult, _ := callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_get_structured_content"})
	elementsResult, _ := callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_mark_elements"})
	return fmt.Sprintf("%s\n[Page content]: %s\n[Interactive elements]: %s", screenshotResult, contentResult, elementsResult), nil
}

// ─── Scroll ──────────────────────────────────────────

type scrollSkill struct{ ctrl BrowserController }

func (s *scrollSkill) Name() string        { return "browser_scroll" }
func (s *scrollSkill) Description() string  { return "Scroll the browser page (up/down/left/right). Set to_end=true to scroll to the very end." }
func (s *scrollSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{
		{Name: "direction", Type: "string", Desc: "up, down, left, or right", Required: true},
		{Name: "to_end", Type: "boolean", Desc: "Scroll to the very end"},
	})
}
func (s *scrollSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	dir, _ := args["direction"].(string)
	toEnd, _ := args["to_end"].(bool)
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_scroll", "direction": dir, "to_end": toEnd})
}

// ─── Get Content ─────────────────────────────────────

type getContentSkill struct{ ctrl BrowserController }

func (s *getContentSkill) Name() string        { return "browser_get_content" }
func (s *getContentSkill) Description() string  { return "Extract the text content of the current browser page." }
func (s *getContentSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *getContentSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_get_content"})
}

// ─── Press Key ───────────────────────────────────────

type pressKeySkill struct{ ctrl BrowserController }

func (s *pressKeySkill) Name() string        { return "browser_press_key" }
func (s *pressKeySkill) Description() string  { return "Press a keyboard key or combination (e.g., Enter, Ctrl+C, PageDown)." }
func (s *pressKeySkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{{Name: "key", Type: "string", Desc: "Key or combination", Required: true}})
}
func (s *pressKeySkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	key, _ := args["key"].(string)
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_press_key", "key": key})
}

// ─── Mark Elements ───────────────────────────────────

type markElementsSkill struct{ ctrl BrowserController }

func (s *markElementsSkill) Name() string        { return "browser_mark_elements" }
func (s *markElementsSkill) Description() string  { return "Annotate the current page with numbered markers on all interactive elements. Returns a screenshot with annotations and the element list." }
func (s *markElementsSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *markElementsSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_mark_elements"})
}

// ─── Unmark Elements ─────────────────────────────────

type unmarkElementsSkill struct{ ctrl BrowserController }

func (s *unmarkElementsSkill) Name() string        { return "browser_unmark_elements" }
func (s *unmarkElementsSkill) Description() string  { return "Remove all element annotation markers from the page." }
func (s *unmarkElementsSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *unmarkElementsSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_unmark_elements"})
}

// ─── Get Elements ────────────────────────────────────

type getElementsSkill struct{ ctrl BrowserController }

func (s *getElementsSkill) Name() string        { return "browser_get_elements" }
func (s *getElementsSkill) Description() string  { return "List all interactive elements on the current page with their index, tag, text, and bounding box." }
func (s *getElementsSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *getElementsSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_get_elements"})
}

// ─── List Tabs ───────────────────────────────────────

type listTabsSkill struct{ ctrl BrowserController }

func (s *listTabsSkill) Name() string        { return "browser_list_tabs" }
func (s *listTabsSkill) Description() string  { return "List all open browser tabs with their id, title, URL, and active status." }
func (s *listTabsSkill) Parameters() map[string]any { return jsonSchema(nil) }
func (s *listTabsSkill) Execute(ctx context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_list_tabs"})
}

// ─── Switch Tab ──────────────────────────────────────

type switchTabSkill struct{ ctrl BrowserController }

func (s *switchTabSkill) Name() string        { return "browser_switch_tab" }
func (s *switchTabSkill) Description() string  { return "Switch to a browser tab by its tab ID." }
func (s *switchTabSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{{Name: "tab_id", Type: "number", Desc: "Tab ID to switch to", Required: true}})
}
func (s *switchTabSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	tabID, ok := toFloat(args["tab_id"])
	if !ok {
		return "", fmt.Errorf("tab_id is required")
	}
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_switch_tab", "tabId": int(tabID)})
}

// ─── New Tab ─────────────────────────────────────────

type newTabSkill struct{ ctrl BrowserController }

func (s *newTabSkill) Name() string        { return "browser_new_tab" }
func (s *newTabSkill) Description() string  { return "Open a new browser tab, optionally navigating to a URL." }
func (s *newTabSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{{Name: "url", Type: "string", Desc: "URL to open (optional, defaults to blank)"}})
}
func (s *newTabSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	action := map[string]any{"type": "browser_new_tab"}
	if url, ok := args["url"].(string); ok && url != "" {
		action["url"] = url
	}
	return callBrowser(ctx, s.ctrl, action)
}

// ─── Close Tab ───────────────────────────────────────

type closeTabSkill struct{ ctrl BrowserController }

func (s *closeTabSkill) Name() string        { return "browser_close_tab" }
func (s *closeTabSkill) Description() string  { return "Close a browser tab by its tab ID." }
func (s *closeTabSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{{Name: "tab_id", Type: "number", Desc: "Tab ID to close", Required: true}})
}
func (s *closeTabSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	tabID, ok := toFloat(args["tab_id"])
	if !ok {
		return "", fmt.Errorf("tab_id is required")
	}
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "browser_close_tab", "tabId": int(tabID)})
}

// ─── User Takeover ───────────────────────────────────

type takeoverSkill struct{ ctrl BrowserController }

func (s *takeoverSkill) Name() string        { return "browser_takeover" }
func (s *takeoverSkill) Description() string  { return "Pause AI browser control and hand over to the user. Call with resume=true to resume AI control." }
func (s *takeoverSkill) Parameters() map[string]any {
	return jsonSchema([]paramDef{
		{Name: "resume", Type: "boolean", Desc: "Set to true to resume AI control; false to pause and hand over to user"},
		{Name: "reason", Type: "string", Desc: "Reason for takeover (shown to user)"},
	})
}
func (s *takeoverSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	resume, _ := args["resume"].(bool)
	reason, _ := args["reason"].(string)
	status := "take_over"
	if resume {
		status = "resumed"
	}
	return callBrowser(ctx, s.ctrl, map[string]any{"type": "session_status", "status": status, "sessionTitle": reason})
}

// ─── Helpers ─────────────────────────────────────────

type paramDef struct {
	Name     string
	Type     string
	Desc     string
	Required bool
}

func jsonSchema(params []paramDef) map[string]any {
	props := map[string]any{}
	var required []string
	for _, p := range params {
		props[p.Name] = map[string]any{"type": p.Type, "description": p.Desc}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func callBrowser(ctx context.Context, ctrl BrowserController, action map[string]any) (string, error) {
	if !ctrl.Connected() {
		return "browser extension not connected — please install and connect the Yunque Browser Connector extension", nil
	}
	result, err := ctrl.SendAction(ctx, action)
	if err != nil {
		return "", err
	}
	resultMap, _ := result.(map[string]any)
	if resultMap != nil {
		if _, has := resultMap["screenshot"]; has {
			resultMap["screenshot"] = "[captured]"
		}
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
