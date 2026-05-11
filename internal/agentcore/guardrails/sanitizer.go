package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"golang.org/x/text/unicode/norm"

	"yunque-agent/internal/agentcore/audit"
)

// InputSource identifies where external input came from.
type InputSource string

const (
	SourceUserPrompt InputSource = "user_prompt"
	SourceToolReturn InputSource = "tool_return"
	SourceMCPResponse InputSource = "mcp_response"
	SourceWebhook     InputSource = "webhook"
)

// SanitizeRequest wraps an external input for unified sanitization.
type SanitizeRequest struct {
	Input  string
	Source InputSource
}

// SanitizeResult extends CheckResult with sanitizer-specific metadata.
type SanitizeResult struct {
	CheckResult
	Source       InputSource `json:"source"`
	ThreatType   string     `json:"threat_type,omitempty"`
	Sanitized    string     `json:"sanitized,omitempty"`
	MatchedRules []string   `json:"matched_rules,omitempty"`
}

// SanitizerConfig controls which checks are active.
type SanitizerConfig struct {
	EnableSQLInjection     bool
	EnableXSS              bool
	EnableCommandInjection bool
	EnablePromptInjection  bool
	EnablePathTraversal    bool
	MaxInputLen            int
	CustomBlockPatterns    []string
}

// DefaultSanitizerConfig returns a secure default with all checks enabled.
func DefaultSanitizerConfig() SanitizerConfig {
	return SanitizerConfig{
		EnableSQLInjection:     true,
		EnableXSS:              true,
		EnableCommandInjection: true,
		EnablePromptInjection:  true,
		EnablePathTraversal:    true,
		MaxInputLen:            500_000,
	}
}

// Sanitizer is a unified input sanitization middleware that filters all
// external inputs (user prompts, tool returns, MCP responses) through
// a single security boundary. It covers the "sanitization layer" of a
// four-layer security architecture (sandbox → permissions → sanitization → audit).
type Sanitizer struct {
	config SanitizerConfig
	audit  *audit.Chain
}

// NewSanitizer creates a Sanitizer with the given config.
func NewSanitizer(cfg SanitizerConfig) *Sanitizer {
	return &Sanitizer{config: cfg}
}

// SetAudit attaches an audit chain for logging blocked/sanitized events.
func (s *Sanitizer) SetAudit(chain *audit.Chain) { s.audit = chain }

func (s *Sanitizer) Name() string { return "sanitizer" }

// --- SQL injection patterns ---

var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\b(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|EXEC|EXECUTE|UNION)\b\s+(ALL\s+)?)`),
	regexp.MustCompile(`(?i)'\s*(OR|AND)\s+['"\d]`),
	regexp.MustCompile(`(?i)(--|#|/\*)\s*$`),
	regexp.MustCompile(`(?i)\bOR\s+1\s*=\s*1\b`),
	regexp.MustCompile(`(?i)\bAND\s+1\s*=\s*1\b`),
	regexp.MustCompile(`(?i);\s*(DROP|DELETE|INSERT|UPDATE|ALTER)\b`),
	regexp.MustCompile(`(?i)\bUNION\s+(ALL\s+)?SELECT\b`),
	regexp.MustCompile(`(?i)\bINTO\s+(OUT|DUMP)FILE\b`),
	regexp.MustCompile(`(?i)\bLOAD_FILE\s*\(`),
	regexp.MustCompile(`(?i)\bWAITFOR\s+DELAY\b`),
	regexp.MustCompile(`(?i)\bBENCHMARK\s*\(`),
	regexp.MustCompile(`(?i)\bSLEEP\s*\(\s*\d+\s*\)`),
	regexp.MustCompile(`(?i)0x[0-9a-f]{8,}`),
}

// --- XSS patterns ---

var xssPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<\s*script\b[^>]*>`),
	regexp.MustCompile(`(?i)\bon\w+\s*=\s*["']`),
	regexp.MustCompile(`(?i)javascript\s*:`),
	regexp.MustCompile(`(?i)vbscript\s*:`),
	regexp.MustCompile(`(?i)data\s*:\s*text/html`),
	regexp.MustCompile(`(?i)<\s*iframe\b[^>]*>`),
	regexp.MustCompile(`(?i)<\s*object\b[^>]*>`),
	regexp.MustCompile(`(?i)<\s*embed\b[^>]*>`),
	regexp.MustCompile(`(?i)<\s*img\b[^>]*\bon(error|load)\s*=`),
	regexp.MustCompile(`(?i)<\s*svg\b[^>]*\bon\w+\s*=`),
	regexp.MustCompile(`(?i)expression\s*\(`),
	regexp.MustCompile(`(?i)url\s*\(\s*['"]?\s*javascript\s*:`),
}

// --- Command injection patterns ---

var commandInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile("(?i)`[^`]*`"),
	regexp.MustCompile(`\$\([^)]+\)`),
	regexp.MustCompile(`(?i);\s*(rm|cat|wget|curl|nc|ncat|bash|sh|python|perl|ruby|php)\b`),
	regexp.MustCompile(`(?i)\|\s*(rm|cat|wget|curl|nc|ncat|bash|sh|python|perl|ruby|php)\b`),
	regexp.MustCompile(`(?i)&&\s*(rm|cat|wget|curl|nc|ncat|bash|sh|python|perl|ruby|php)\b`),
	regexp.MustCompile(`(?i)\b(rm\s+-rf|chmod\s+777|chown\s+root)\b`),
	regexp.MustCompile(`(?i)/etc/(passwd|shadow|hosts)`),
	regexp.MustCompile(`(?i)\b(nc|ncat|netcat)\s+-[elp]`),
	regexp.MustCompile(`(?i)\b(wget|curl)\s+.{5,}\s*\|\s*(bash|sh)\b`),
	// PowerShell-specific vectors
	regexp.MustCompile(`(?i)\bInvoke-Expression\b`),
	regexp.MustCompile(`(?i)\biex\s+\S`),
	regexp.MustCompile(`(?i)\bStart-Process\b`),
	regexp.MustCompile(`(?i)\bNew-Object\b.*\bNet\.WebClient\b`),
	regexp.MustCompile(`(?i)\[System\.Net\.WebClient\]`),
	regexp.MustCompile(`(?i)\.DownloadString\s*\(`),
	regexp.MustCompile(`(?i)\.DownloadFile\s*\(`),
	regexp.MustCompile(`(?i)\b(?:powershell|pwsh)(?:\.exe)?\b.*\s-e(?:nc(?:odedcommand)?)?\b`),
	regexp.MustCompile(`(?i)\bSet-ExecutionPolicy\b`),
	regexp.MustCompile(`(?i)\bAdd-Type\b.*-TypeDefinition\b`),
}

// --- Path traversal patterns ---

var pathTraversalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(\.\.[\\/]){2,}`),
	regexp.MustCompile(`(?i)%2e%2e[%/\\]`),
	regexp.MustCompile(`(?i)\.\.%2f`),
	regexp.MustCompile(`(?i)%2e%2e%2f`),
	regexp.MustCompile(`(?i)\.\.%5c`),
	regexp.MustCompile(`(?i)%252e%252e`),
}

// sqlBlockCommentRegex matches SQL block comments used to evade keyword detection
// (e.g., S/**/E/**/L/**/E/**/C/**/T → SELECT after stripping).
var sqlBlockCommentRegex = regexp.MustCompile(`/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`)

// normalizeForDetection applies NFKC normalization (collapses fullwidth chars,
// Unicode homoglyphs) and strips SQL block comments. The result is used only
// for pattern matching; the original input is preserved for output/sanitization.
func normalizeForDetection(input string) string {
	out := norm.NFKC.String(input)
	out = sqlBlockCommentRegex.ReplaceAllString(out, "")
	return out
}

// Sanitize validates and sanitizes a single external input.
func (s *Sanitizer) Sanitize(ctx context.Context, req SanitizeRequest) SanitizeResult {
	result := SanitizeResult{
		CheckResult: CheckResult{Passed: true},
		Source:      req.Source,
	}

	input := req.Input

	// Length check
	if s.config.MaxInputLen > 0 && len(input) > s.config.MaxInputLen {
		result.Passed = false
		result.Blocked = true
		result.Rule = "sanitizer_max_length"
		result.ThreatType = "overflow"
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("input exceeds max length (%d > %d)", len(input), s.config.MaxInputLen))
		s.auditEvent("block", "max_length", req.Source, "")
		return result
	}

	// Null byte injection
	if strings.ContainsRune(input, 0) {
		input = strings.ReplaceAll(input, "\x00", "")
		result.Warnings = append(result.Warnings, "null bytes stripped")
		result.MatchedRules = append(result.MatchedRules, "null_byte")
		result.Sanitized = input
		s.auditEvent("sanitize", "null_byte", req.Source, "")
	}

	// Normalize for detection (NFKC + strip SQL block comments)
	detectInput := normalizeForDetection(input)

	// Custom block patterns
	lower := strings.ToLower(detectInput)
	for _, pat := range s.config.CustomBlockPatterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			result.Passed = false
			result.Blocked = true
			result.Rule = "sanitizer_custom_block"
			result.ThreatType = "custom"
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("blocked by custom pattern: %s", pat))
			s.auditEvent("block", "custom_pattern", req.Source, pat)
			return result
		}
	}

	// SQL injection (skip for pure user prompts in chat mode unless embedded in tool args)
	if s.config.EnableSQLInjection && req.Source != SourceUserPrompt {
		if threat := matchPatterns(detectInput, sqlInjectionPatterns); threat != "" {
			result.Passed = false
			result.Blocked = true
			result.Rule = "sanitizer_sql_injection"
			result.ThreatType = "sql_injection"
			result.MatchedRules = append(result.MatchedRules, "sql_injection")
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("SQL injection pattern detected: %s", truncate(threat, 60)))
			s.auditEvent("block", "sql_injection", req.Source, truncate(threat, 60))
			return result
		}
	}

	// XSS
	if s.config.EnableXSS {
		if threat := matchPatterns(detectInput, xssPatterns); threat != "" {
			sanitized := sanitizeXSS(input)
			result.Sanitized = sanitized
			result.ThreatType = "xss"
			result.MatchedRules = append(result.MatchedRules, "xss")
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("XSS pattern sanitized: %s", truncate(threat, 60)))
			s.auditEvent("sanitize", "xss", req.Source, truncate(threat, 60))
			input = sanitized
		}
	}

	// Command injection (only for tool/MCP inputs)
	if s.config.EnableCommandInjection && (req.Source == SourceToolReturn || req.Source == SourceMCPResponse) {
		if threat := matchPatterns(detectInput, commandInjectionPatterns); threat != "" {
			result.Passed = false
			result.Blocked = true
			result.Rule = "sanitizer_command_injection"
			result.ThreatType = "command_injection"
			result.MatchedRules = append(result.MatchedRules, "command_injection")
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("command injection detected: %s", truncate(threat, 60)))
			s.auditEvent("block", "command_injection", req.Source, truncate(threat, 60))
			return result
		}
	}

	// Path traversal
	if s.config.EnablePathTraversal {
		if threat := matchPatterns(detectInput, pathTraversalPatterns); threat != "" {
			result.Passed = false
			result.Blocked = true
			result.Rule = "sanitizer_path_traversal"
			result.ThreatType = "path_traversal"
			result.MatchedRules = append(result.MatchedRules, "path_traversal")
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("path traversal detected: %s", truncate(threat, 60)))
			s.auditEvent("block", "path_traversal", req.Source, truncate(threat, 60))
			return result
		}
	}

	if result.Sanitized == "" && input != req.Input {
		result.Sanitized = input
	}

	return result
}

// Check implements the Guard interface for Pipeline compatibility.
// Uses SourceUserPrompt as default source.
func (s *Sanitizer) Check(ctx context.Context, input string) CheckResult {
	sr := s.Sanitize(ctx, SanitizeRequest{Input: input, Source: SourceUserPrompt})
	r := sr.CheckResult
	if sr.Sanitized != "" {
		r.Redacted = sr.Sanitized
	}
	return r
}

// --- helpers ---

func matchPatterns(input string, patterns []*regexp.Regexp) string {
	for _, p := range patterns {
		if m := p.FindString(input); m != "" {
			return m
		}
	}
	return ""
}

var htmlEscaper = strings.NewReplacer(
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&#39;",
)

func sanitizeXSS(input string) string {
	return htmlEscaper.Replace(input)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *Sanitizer) auditEvent(action, subAction string, source InputSource, detail string) {
	if s.audit == nil {
		return
	}
	s.audit.Append(audit.EventSystem, "sanitizer",
		fmt.Sprintf("%s:%s:%s", action, subAction, source), detail)
}
