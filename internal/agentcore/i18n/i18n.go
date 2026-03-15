// Package i18n provides a simple internationalization system.
// Messages are organized by locale and key, supporting parameter interpolation.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Default locales.
const (
	LocaleZH = "zh"
	LocaleEN = "en"
)

// Bundle holds all locale message maps.
type Bundle struct {
	mu       sync.RWMutex
	locales  map[string]map[string]string // locale -> key -> message
	fallback string
}

// NewBundle creates an i18n bundle with the given fallback locale.
func NewBundle(fallback string) *Bundle {
	if fallback == "" {
		fallback = LocaleZH
	}
	return &Bundle{
		locales:  make(map[string]map[string]string),
		fallback: fallback,
	}
}

// SetFallback changes the fallback locale.
func (b *Bundle) SetFallback(locale string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallback = locale
}

// LoadJSON loads messages from a JSON file: {"key": "message", ...}
func (b *Bundle) LoadJSON(locale, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("i18n load %s: %w", path, err)
	}
	var msgs map[string]string
	if err := json.Unmarshal(data, &msgs); err != nil {
		return fmt.Errorf("i18n parse %s: %w", path, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.locales[locale] == nil {
		b.locales[locale] = make(map[string]string)
	}
	for k, v := range msgs {
		b.locales[locale][k] = v
	}
	return nil
}

// LoadDir loads all JSON files in a directory, using filename (without ext) as locale.
// e.g. zh.json -> locale "zh", en.json -> locale "en"
func (b *Bundle) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		locale := strings.TrimSuffix(e.Name(), ".json")
		if err := b.LoadJSON(locale, filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Set sets a message for a locale and key.
func (b *Bundle) Set(locale, key, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.locales[locale] == nil {
		b.locales[locale] = make(map[string]string)
	}
	b.locales[locale][key] = message
}

// T translates a key for the given locale with optional format args.
// Falls back to the fallback locale if key not found.
// Falls back to the key itself if still not found.
func (b *Bundle) T(locale, key string, args ...any) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Try requested locale
	if msgs, ok := b.locales[locale]; ok {
		if msg, ok := msgs[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// Try fallback locale
	if locale != b.fallback {
		if msgs, ok := b.locales[b.fallback]; ok {
			if msg, ok := msgs[key]; ok {
				if len(args) > 0 {
					return fmt.Sprintf(msg, args...)
				}
				return msg
			}
		}
	}

	// Return key as-is
	return key
}

// HasLocale reports whether a locale is loaded.
func (b *Bundle) HasLocale(locale string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.locales[locale]
	return ok
}

// Locales returns all loaded locale names.
func (b *Bundle) Locales() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]string, 0, len(b.locales))
	for k := range b.locales {
		result = append(result, k)
	}
	return result
}

// Keys returns all message keys for a locale.
func (b *Bundle) Keys(locale string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	msgs, ok := b.locales[locale]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(msgs))
	for k := range msgs {
		result = append(result, k)
	}
	return result
}

// Count returns the number of keys for a locale.
func (b *Bundle) Count(locale string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.locales[locale])
}

// --- Default bundle + convenience functions ---

var defaultBundle = NewBundle(LocaleZH)

func init() {
	// Register built-in Chinese messages
	defaults := map[string]string{
		"agent.greeting":           "你好，我是云鸢智能助手。有什么可以帮你的？",
		"agent.error":              "抱歉，处理你的请求时出现了问题。",
		"agent.thinking":           "正在思考...",
		"agent.skill_not_found":    "未找到技能: %s",
		"agent.loop_detected":      "检测到对话循环，请尝试换个方式提问。",
		"agent.rate_limited":       "请求过于频繁，请稍后再试。",
		"agent.unauthorized":       "身份验证失败，请重新登录。",
		"agent.context_too_long":   "上下文过长，已自动裁剪历史消息。",
		"bot.created":              "Bot [%s] 已创建。",
		"bot.deleted":              "Bot [%s] 已删除。",
		"bot.not_found":            "未找到 Bot: %s",
		"channel.connected":        "%s 渠道已连接。",
		"channel.disconnected":     "%s 渠道已断开。",
		"channel.error":            "%s 渠道出错: %s",
		"memory.saved":             "记忆已保存。",
		"memory.not_found":         "未找到相关记忆。",
		"setup.welcome":            "欢迎使用云鸢 Agent 配置向导。",
		"setup.complete":           "配置完成，可以启动 Agent 了。",
		"doctor.check_pass":        "[OK] %s",
		"doctor.check_warn":        "[WARN] %s",
		"doctor.check_fail":        "[FAIL] %s",
	}
	for k, v := range defaults {
		defaultBundle.Set(LocaleZH, k, v)
	}

	// Register built-in English messages
	defaultsEN := map[string]string{
		"agent.greeting":           "Hello, I'm Yunque AI Assistant. How can I help you?",
		"agent.error":              "Sorry, there was a problem processing your request.",
		"agent.thinking":           "Thinking...",
		"agent.skill_not_found":    "Skill not found: %s",
		"agent.loop_detected":      "Conversational loop detected. Please try rephrasing.",
		"agent.rate_limited":       "Too many requests. Please try again later.",
		"agent.unauthorized":       "Authentication failed. Please log in again.",
		"agent.context_too_long":   "Context too long; older messages have been trimmed.",
		"bot.created":              "Bot [%s] created.",
		"bot.deleted":              "Bot [%s] deleted.",
		"bot.not_found":            "Bot not found: %s",
		"channel.connected":        "%s channel connected.",
		"channel.disconnected":     "%s channel disconnected.",
		"channel.error":            "%s channel error: %s",
		"memory.saved":             "Memory saved.",
		"memory.not_found":         "No relevant memories found.",
		"setup.welcome":            "Welcome to Yunque Agent setup wizard.",
		"setup.complete":           "Setup complete. You can start the Agent now.",
		"doctor.check_pass":        "[OK] %s",
		"doctor.check_warn":        "[WARN] %s",
		"doctor.check_fail":        "[FAIL] %s",
	}
	for k, v := range defaultsEN {
		defaultBundle.Set(LocaleEN, k, v)
	}
}

// Default returns the default bundle.
func Default() *Bundle { return defaultBundle }

// T translates using the default bundle.
func T(locale, key string, args ...any) string {
	return defaultBundle.T(locale, key, args...)
}
