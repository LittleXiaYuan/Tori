package planner

import (
	"context"
	"log/slog"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/llm"
)

// ContextWindowRuntimeService owns Planner's context compression and hard
// window-trimming runtime. Planner still decides which messages to build, but
// this service decides how to fit them into the selected model window.
type ContextWindowRuntimeService struct {
	windowCfg *ctxwindow.WindowConfig
	manager   *ctxwindow.Manager
}

func NewContextWindowRuntimeService() *ContextWindowRuntimeService {
	return &ContextWindowRuntimeService{}
}

func (s *ContextWindowRuntimeService) SetWindowConfig(cfg ctxwindow.WindowConfig) {
	if s == nil {
		return
	}
	s.windowCfg = &cfg
}

func (s *ContextWindowRuntimeService) WindowConfig() *ctxwindow.WindowConfig {
	if s == nil || s.windowCfg == nil {
		return nil
	}
	cfg := *s.windowCfg
	return &cfg
}

func (s *ContextWindowRuntimeService) SetManager(mgr *ctxwindow.Manager) {
	if s == nil {
		return
	}
	s.manager = mgr
}

func (s *ContextWindowRuntimeService) Manager() *ctxwindow.Manager {
	if s == nil {
		return nil
	}
	return s.manager
}

func (s *ContextWindowRuntimeService) PruneToolResults(messages []llm.Message, maxBytes int) {
	for i := range messages {
		if messages[i].Role == "tool" && len(messages[i].Content) > maxBytes {
			messages[i].Content = ctxwindow.PruneToolOutput(messages[i].Content, maxBytes)
		}
	}
}

func (s *ContextWindowRuntimeService) CompressAndTrim(ctx context.Context, messages []llm.Message, client *llm.Client) []llm.Message {
	messages = s.compress(ctx, messages)
	return s.trim(messages, client)
}

func (s *ContextWindowRuntimeService) FitMessagesForRequest(ctx context.Context, messages []llm.Message, client *llm.Client) []llm.Message {
	s.PruneToolResults(messages, 6000)
	return s.CompressAndTrim(ctx, messages, client)
}

func (s *ContextWindowRuntimeService) compress(ctx context.Context, messages []llm.Message) []llm.Message {
	if s == nil || s.manager == nil {
		return messages
	}
	winMsgs := plannerMessagesToWindowMessages(messages)
	compressed, err := s.manager.Process(ctx, winMsgs)
	if err != nil {
		slog.Warn("context compression failed, falling back to window trim", "err", err)
		return messages
	}
	if len(compressed) >= len(messages) {
		return messages
	}
	slog.Info("context compressed", "before", len(messages), "after", len(compressed))
	return windowMessagesToPlannerMessages(compressed)
}

func (s *ContextWindowRuntimeService) trim(messages []llm.Message, client *llm.Client) []llm.Message {
	windowCfg := s.effectiveWindowConfig(client)
	if windowCfg == nil {
		return messages
	}
	result := ctxwindow.TrimToFit(plannerMessagesToWindowMessages(messages), *windowCfg)
	if result.DroppedCount <= 0 {
		return messages
	}
	slog.Info("context window trimmed", "dropped", result.DroppedCount, "remaining", len(result.Messages), "model_window_k", windowCfg.MaxTokens/1024)
	return windowMessagesToPlannerMessages(result.Messages)
}

func (s *ContextWindowRuntimeService) effectiveWindowConfig(client *llm.Client) *ctxwindow.WindowConfig {
	var windowCfg *ctxwindow.WindowConfig
	if s != nil && s.windowCfg != nil {
		cfg := *s.windowCfg
		windowCfg = &cfg
	}
	if client != nil {
		modelTokens := client.ContextWindowTokens()
		if windowCfg == nil || modelTokens != windowCfg.MaxTokens {
			cfg := ctxwindow.ConfigForWindow(modelTokens / 1024)
			windowCfg = &cfg
		}
	}
	return windowCfg
}

func plannerMessagesToWindowMessages(messages []llm.Message) []ctxwindow.Message {
	winMsgs := make([]ctxwindow.Message, len(messages))
	for i, m := range messages {
		winMsgs[i] = ctxwindow.Message{Role: m.Role, Content: m.Content}
	}
	return winMsgs
}

func windowMessagesToPlannerMessages(messages []ctxwindow.Message) []llm.Message {
	trimmed := make([]llm.Message, len(messages))
	for i, m := range messages {
		trimmed[i] = llm.Message{Role: m.Role, Content: m.Content}
	}
	return trimmed
}
