package cogni

import (
	"context"
	"testing"
)

func TestNLConfigTranslator_SchedulerCreate(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return `{
			"intent": "scheduler_create",
			"confidence": 0.95,
			"summary": "创建每天检查系统状态的定时任务",
			"params": {
				"name": "系统状态检查",
				"prompt": "请检查当前系统运行状态，包括CPU、内存、磁盘使用率，如有异常请生成报告",
				"interval": "24h"
			}
		}`, nil
	})

	result, err := translator.Translate(context.Background(), NLConfigRequest{
		Text: "帮我创建一个每天检查系统状态的定时任务",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != IntentSchedulerCreate {
		t.Errorf("expected intent scheduler_create, got %s", result.Intent)
	}
	if result.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", result.Confidence)
	}

	sp, err := ParseSchedulerParams(result.Params)
	if err != nil {
		t.Fatalf("parse scheduler params: %v", err)
	}
	if sp.Name != "系统状态检查" {
		t.Errorf("expected name '系统状态检查', got %q", sp.Name)
	}
	if sp.Interval != "24h" {
		t.Errorf("expected interval '24h', got %q", sp.Interval)
	}
}

func TestNLConfigTranslator_KBAdd(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return `{
			"intent": "kb_add",
			"confidence": 0.92,
			"summary": "添加退款政策到知识库",
			"params": {
				"name": "退款政策",
				"content": "7天无理由退款，需要商品完好。超过7天需要联系客服处理。",
				"trigger": "当用户询问退款相关问题时"
			}
		}`, nil
	})

	result, err := translator.Translate(context.Background(), NLConfigRequest{
		Text: "记住我们的退款政策：7天无理由退款，需要商品完好。超过7天需要联系客服处理。",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != IntentKBAdd {
		t.Errorf("expected intent kb_add, got %s", result.Intent)
	}

	kp, err := ParseKBParams(result.Params)
	if err != nil {
		t.Fatalf("parse kb params: %v", err)
	}
	if kp.Name != "退款政策" {
		t.Errorf("expected name '退款政策', got %q", kp.Name)
	}
	if kp.Trigger == "" {
		t.Error("expected non-empty trigger")
	}
}

func TestNLConfigTranslator_LowConfidence(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return `{
			"intent": "unknown",
			"confidence": 0.2,
			"summary": "无法识别用户意图",
			"params": {}
		}`, nil
	})

	result, err := translator.Translate(context.Background(), NLConfigRequest{
		Text: "今天天气怎么样",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != IntentUnknown {
		t.Errorf("expected intent unknown, got %s", result.Intent)
	}
}

func TestNLConfigTranslator_EmptyInput(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return "", nil
	})

	_, err := translator.Translate(context.Background(), NLConfigRequest{Text: ""})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestNLConfigTranslator_NilLLM(t *testing.T) {
	translator := NewNLConfigTranslator(nil)
	_, err := translator.Translate(context.Background(), NLConfigRequest{Text: "test"})
	if err == nil {
		t.Fatal("expected error for nil LLM")
	}
}

func TestNLConfigTranslator_CogniCreate(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return `{
			"intent": "cogni_create",
			"confidence": 0.88,
			"summary": "创建一个翻译助手智体",
			"params": {
				"description": "创建一个中英文翻译助手，支持技术文档翻译"
			}
		}`, nil
	})

	result, err := translator.Translate(context.Background(), NLConfigRequest{
		Text: "创建一个翻译助手，能做中英文翻译",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != IntentCogniCreate {
		t.Errorf("expected intent cogni_create, got %s", result.Intent)
	}
}

func TestParseSchedulerParams_MissingName(t *testing.T) {
	_, err := ParseSchedulerParams(map[string]any{
		"prompt":   "test",
		"interval": "1h",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseSchedulerParams_InvalidInterval(t *testing.T) {
	_, err := ParseSchedulerParams(map[string]any{
		"name":     "test",
		"prompt":   "test",
		"interval": "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}
}

func TestParseSchedulerParams_DefaultInterval(t *testing.T) {
	sp, err := ParseSchedulerParams(map[string]any{
		"name":   "test",
		"prompt": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.Interval != "1h" {
		t.Errorf("expected default interval '1h', got %q", sp.Interval)
	}
}

func TestSupportedIntents(t *testing.T) {
	intents := SupportedIntents()
	if len(intents) < 9 {
		t.Errorf("expected at least 9 intents, got %d", len(intents))
	}
}

func TestNLConfigTranslator_MarkdownCodeBlock(t *testing.T) {
	translator := NewNLConfigTranslator(func(_ context.Context, system, user string) (string, error) {
		return "```json\n" + `{
			"intent": "scheduler_list",
			"confidence": 0.99,
			"summary": "查看所有定时任务",
			"params": {}
		}` + "\n```", nil
	})

	result, err := translator.Translate(context.Background(), NLConfigRequest{
		Text: "看看我有哪些定时任务",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent != IntentSchedulerList {
		t.Errorf("expected intent scheduler_list, got %s", result.Intent)
	}
}
