package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"yunque-agent/internal/agentcore/i18n"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

// PromptRuntimeService owns system-prompt runtime configuration and cache.
//
// This keeps locale, persona/domain prompt overlays, native-function-calling
// mode, the L2 skill index, and prompt cache state out of Planner while
// preserving the same public Planner setters.
type PromptRuntimeService struct {
	domainPrompt    string
	personaPrompt   func() string
	nativeFC        bool
	cachedSysPrompt string
	sysPromptVer    int
	skillIndex      SkillIndexFunc
	locale          string
}

func NewPromptRuntimeService() *PromptRuntimeService {
	return &PromptRuntimeService{}
}

func (s *PromptRuntimeService) SetPersonaPrompt(fn func() string) {
	if s != nil {
		s.personaPrompt = fn
	}
}

func (s *PromptRuntimeService) PersonaPrompt() string {
	if s == nil || s.personaPrompt == nil {
		return ""
	}
	return s.personaPrompt()
}

func (s *PromptRuntimeService) SetDomainPrompt(prompt string) {
	if s != nil {
		s.domainPrompt = prompt
	}
}

func (s *PromptRuntimeService) DomainPrompt() string {
	if s == nil {
		return ""
	}
	return s.domainPrompt
}

func (s *PromptRuntimeService) SetNativeFC(enabled bool) {
	if s == nil {
		return
	}
	s.nativeFC = enabled
	s.InvalidatePromptCache()
}

func (s *PromptRuntimeService) NativeFC() bool {
	return s != nil && s.nativeFC
}

func (s *PromptRuntimeService) SetSkillIndex(fn SkillIndexFunc) {
	if s != nil {
		s.skillIndex = fn
	}
}

func (s *PromptRuntimeService) SetLocale(locale string) {
	if s != nil {
		s.locale = locale
	}
}

func (s *PromptRuntimeService) Locale() string {
	if s == nil || s.locale == "" {
		return "zh-CN"
	}
	return s.locale
}

func (s *PromptRuntimeService) Translate(key string, args ...any) string {
	return i18n.T(s.Locale(), key, args...)
}

func (s *PromptRuntimeService) ReflectRetryPrompt() string {
	return s.Translate("planner.reflect_retry")
}

func (s *PromptRuntimeService) TaskStoppedReply() string {
	return s.Translate("planner.task_stopped")
}

func (s *PromptRuntimeService) InvalidatePromptCache() {
	if s != nil {
		s.sysPromptVer++
	}
}

func (s *PromptRuntimeService) BuildSystemPrompt(registry *skills.Registry) string {
	if registry == nil {
		return "You are an AI assistant."
	}
	currentVer := registry.Version()
	if s != nil && s.cachedSysPrompt != "" && s.sysPromptVer == currentVer {
		return s.cachedSysPrompt
	}

	content, loc := s.readPromptTemplate()
	tmpl, err := template.New("system").Parse(string(content))
	if err != nil {
		slog.Error("planner: failed to parse prompt template", "err", err)
		return string(content)
	}

	defsJSON, _ := json.MarshalIndent(registry.Definitions(), "", "  ")
	var indexItems []SkillIndexEntry
	if s != nil && s.skillIndex != nil {
		indexItems = s.skillIndex()
	}

	categories := registry.Categories()
	catMap := make(map[string]bool, len(categories))
	for _, c := range categories {
		catMap[c.ID] = len(c.SkillNames) > 0
	}

	data := map[string]any{
		"SkillDefinitions": string(defsJSON),
		"SkillIndex":       indexItems,
		"NativeFC":         s != nil && s.nativeFC,
		"Categories":       catMap,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("planner: failed to execute prompt template", "err", err)
		return string(content)
	}

	prompt := buf.String()
	if s != nil {
		s.cachedSysPrompt = prompt
		s.sysPromptVer = currentVer
	}
	hasBrowserSection := strings.Contains(prompt, "browser_navigate")
	slog.Info("buildSystemPrompt", "len", len(prompt), "has_browser", hasBrowserSection, "catMap", catMap, "ver", currentVer, "locale", loc)
	return prompt
}

func (s *PromptRuntimeService) BuildSubagentSystemPrompt(registry *skills.Registry) string {
	if registry == nil {
		return "You are an AI execution agent. Use the provided tools to complete the task."
	}
	content, _ := s.readPromptTemplate()
	if len(content) == 0 {
		return "You are an AI execution agent. Use the provided tools to complete the task."
	}

	tmpl, err := template.New("subagent").Parse(string(content))
	if err != nil {
		return string(content)
	}

	defsJSON, _ := json.MarshalIndent(registry.Definitions(), "", "  ")
	data := map[string]any{
		"SkillDefinitions": string(defsJSON),
		"SkillIndex":       []SkillIndexEntry{},
		"NativeFC":         false,
		"Categories":       map[string]bool{},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return string(content)
	}
	return buf.String()
}

func (s *PromptRuntimeService) BuildStablePrefix(disableDelegation bool, groupSystemPrompt string, buildDefault, buildSubagent func() string) string {
	var stablePrefix string
	if pp := s.PersonaPrompt(); pp != "" {
		stablePrefix = pp + "\n\n"
	}

	if disableDelegation {
		if buildSubagent != nil {
			stablePrefix += buildSubagent()
		}
	} else if buildDefault != nil {
		stablePrefix += buildDefault()
	}

	if domainPrompt := s.DomainPrompt(); domainPrompt != "" {
		stablePrefix += "\n\n" + domainPrompt
	}
	if groupSystemPrompt != "" {
		stablePrefix += "\n\n" + groupSystemPrompt
	}
	return stablePrefix
}

func (s *PromptRuntimeService) PrepareConversationMessages(messages []llm.Message, now time.Time) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	convMsgs := make([]llm.Message, len(messages))
	copy(convMsgs, messages)

	for i := len(convMsgs) - 1; i >= 0; i-- {
		if convMsgs[i].Role != "user" {
			continue
		}
		ts := fmt.Sprintf("[时间: %s]\n", now.Format("2006-01-02 15:04"))
		if len(convMsgs[i].ContentParts) > 0 {
			updated := convMsgs[i]
			parts := make([]llm.ContentPart, len(updated.ContentParts))
			copy(parts, updated.ContentParts)
			prefixed := false
			for j := range parts {
				if parts[j].Type == "text" && !prefixed {
					parts[j].Text = ts + parts[j].Text
					prefixed = true
				}
			}
			if !prefixed {
				parts = append([]llm.ContentPart{{Type: "text", Text: ts}}, parts...)
			}
			updated.ContentParts = parts
			updated.Content = ts + updated.Content
			convMsgs[i] = updated
		} else {
			convMsgs[i] = llm.Message{
				Role:    "user",
				Content: ts + convMsgs[i].Content,
			}
		}
		break
	}

	if len(convMsgs) > 2 {
		var firstGoal string
		for _, m := range convMsgs {
			if m.Role == "user" {
				firstGoal = m.Content
				break
			}
		}
		if firstGoal != "" {
			goalRunes := []rune(firstGoal)
			if len(goalRunes) > 100 {
				firstGoal = string(goalRunes[:100]) + "..."
			}
			last := convMsgs[len(convMsgs)-1]
			convMsgs = append(convMsgs[:len(convMsgs)-1],
				llm.Message{Role: "system", Content: "[任务焦点] 用户的核心目标: " + firstGoal},
				last,
			)
		}
	}
	return convMsgs
}

func (s *PromptRuntimeService) readPromptTemplate() ([]byte, string) {
	loc := s.Locale()
	path := fmt.Sprintf("prompts/%s/system.tmpl", loc)
	content, err := promptFiles.ReadFile(path)
	if err == nil {
		return content, loc
	}
	slog.Warn("planner: prompt locale not found, falling back to English", "locale", loc)
	content, err = promptFiles.ReadFile("prompts/en/system.tmpl")
	if err != nil {
		return []byte("You are an AI assistant. Use the provided tools."), "en"
	}
	return content, "en"
}
