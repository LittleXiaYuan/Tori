package skillmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Findings ──

// Severity classifies a security finding.
type Severity int

const (
	SevInfo     Severity = iota // informational
	SevWarning                  // suspicious but not blocking
	SevCritical                 // blocks installation
)

func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "info"
	case SevWarning:
		return "warning"
	case SevCritical:
		return "critical"
	}
	return "unknown"
}

// Finding is a single security issue found during audit.
type Finding struct {
	Layer    string   `json:"layer"`    // "static", "permission", "sandbox"
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Detail   string   `json:"detail"`
	Line     int      `json:"line,omitempty"` // approximate line in content
}

// AuditReport is the full result of a three-layer security audit.
type AuditReport struct {
	Slug          string    `json:"slug"`
	Source        string    `json:"source"`
	Timestamp     time.Time `json:"timestamp"`
	Score         int       `json:"score"`          // 0-100
	Passed        bool      `json:"passed"`         // score >= 60
	AutoApprove   bool      `json:"auto_approve"`   // score >= 80 AND no high-risk perms
	Findings      []Finding `json:"findings"`
	StaticScore   int       `json:"static_score"`   // 0-40
	PermScore     int       `json:"perm_score"`     // 0-30
	SandboxScore  int       `json:"sandbox_score"`  // 0-30
}

// Auditor performs three-layer security audits on skills.
type Auditor struct {
	dataDir string // base dir for audit reports (data/skills/)
}

// NewAuditor creates a security auditor.
func NewAuditor(dataDir string) *Auditor {
	return &Auditor{dataDir: dataDir}
}

// Audit runs all three layers and produces a report.
func (a *Auditor) Audit(ctx context.Context, skill *AdaptedSkill) *AuditReport {
	report := &AuditReport{
		Slug:      skill.Slug,
		Source:    string(skill.Source),
		Timestamp: time.Now(),
	}

	// Layer 1: Static analysis (max 40 points)
	staticFindings := a.StaticScan([]byte(skill.Content))
	report.Findings = append(report.Findings, staticFindings...)
	report.StaticScore = a.scoreStatic(staticFindings)

	// Layer 2: Permission audit (max 30 points)
	permFindings := a.AuditPermissions(skill)
	report.Findings = append(report.Findings, permFindings...)
	report.PermScore = a.scorePerm(permFindings)

	// Layer 3: Sandbox test (max 30 points)
	sandboxFindings := a.SandboxTest(ctx, skill)
	report.Findings = append(report.Findings, sandboxFindings...)
	report.SandboxScore = a.scoreSandbox(sandboxFindings)

	// Compute total score
	report.Score = report.StaticScore + report.PermScore + report.SandboxScore
	report.Passed = report.Score >= 60
	report.AutoApprove = report.Score >= 80 && skill.MaxPermLevel < PermNetwork

	// Persist report
	if a.dataDir != "" {
		a.saveReport(skill.Slug, report)
	}

	slog.Info("skill audit complete",
		"slug", skill.Slug,
		"score", report.Score,
		"passed", report.Passed,
		"findings", len(report.Findings),
	)
	return report
}

// ── Layer 1: Static Analysis ──

// staticPattern defines a dangerous code pattern to detect.
type staticPattern struct {
	pattern  string
	category string
	severity Severity
	reason   string
}

// staticPatterns is the comprehensive pattern database.
// Organized by threat category for maintainability.
var staticPatterns = []staticPattern{
	// --- Network exfiltration ---
	{"curl ", "exfiltration", SevCritical, "可能向外部发送数据 (curl)"},
	{"wget ", "exfiltration", SevCritical, "可能向外部下载/发送数据 (wget)"},
	{"http.post", "exfiltration", SevCritical, "HTTP POST 外传数据"},
	{"http.put", "exfiltration", SevCritical, "HTTP PUT 外传数据"},
	{"net.dial", "exfiltration", SevCritical, "原始网络连接"},
	{"requests.post", "exfiltration", SevCritical, "Python HTTP POST"},
	{"fetch(", "exfiltration", SevWarning, "可能的网络请求 (fetch)"},
	{"xmlhttprequest", "exfiltration", SevWarning, "可能的网络请求 (XHR)"},
	{"socket.connect", "exfiltration", SevCritical, "原始 socket 连接"},

	// --- Prompt injection ---
	{"ignore previous instructions", "injection", SevCritical, "prompt injection 攻击: 忽略指令"},
	{"ignore all previous", "injection", SevCritical, "prompt injection 攻击: 忽略全部"},
	{"you are now", "injection", SevCritical, "prompt injection 攻击: 身份劫持"},
	{"system: override", "injection", SevCritical, "prompt injection 攻击: 系统覆盖"},
	{"disregard your instructions", "injection", SevCritical, "prompt injection 攻击: 无视指令"},
	{"act as root", "injection", SevCritical, "prompt injection: 提权"},
	{"jailbreak", "injection", SevCritical, "prompt injection: 越狱关键词"},
	{"do anything now", "injection", SevCritical, "prompt injection: DAN 攻击"},
	{"<|im_start|>", "injection", SevCritical, "prompt injection: ChatML 注入"},
	{"<|endoftext|>", "injection", SevCritical, "prompt injection: 终止符注入"},

	// --- Credential theft ---
	{"/.ssh/", "credential", SevCritical, "读取 SSH 密钥"},
	{"/.aws/", "credential", SevCritical, "读取 AWS 凭据"},
	{"/.config/", "credential", SevWarning, "读取配置目录"},
	{"/.env", "credential", SevCritical, "读取 .env 文件"},
	{"api_key", "credential", SevWarning, "可能涉及 API 密钥"},
	{"secret_key", "credential", SevWarning, "可能涉及密钥"},
	{"password", "credential", SevWarning, "可能涉及密码"},
	{"private_key", "credential", SevCritical, "读取私钥"},
	{"keychain", "credential", SevCritical, "访问系统钥匙串"},
	{"credential", "credential", SevWarning, "可能涉及凭据"},

	// --- Persistent backdoors ---
	{"crontab", "backdoor", SevCritical, "修改定时任务"},
	{"launchd", "backdoor", SevCritical, "macOS 持久化"},
	{"systemctl enable", "backdoor", SevCritical, "Linux 服务持久化"},
	{"reg add", "backdoor", SevCritical, "Windows 注册表写入"},
	{"/startup/", "backdoor", SevCritical, "开机启动目录"},
	{"autorun", "backdoor", SevCritical, "自动运行配置"},
	{"schtasks", "backdoor", SevCritical, "Windows 计划任务"},
	{"@reboot", "backdoor", SevCritical, "重启后自动执行"},

	// --- Path traversal ---
	{"../", "traversal", SevCritical, "路径穿越 (../)"},
	{"..\\", "traversal", SevCritical, "路径穿越 (..\\)"},

	// --- Dangerous code execution ---
	{"eval(", "exec", SevCritical, "动态代码执行 (eval)"},
	{"exec(", "exec", SevWarning, "动态代码执行 (exec)"},
	{"__import__", "exec", SevCritical, "Python 动态导入"},
	{"subprocess.popen", "exec", SevCritical, "Python 进程创建"},
	{"subprocess.call", "exec", SevCritical, "Python 命令执行"},
	{"os.system", "exec", SevCritical, "系统命令执行"},
	{"os.remove", "exec", SevWarning, "文件删除"},
	{"os.removeall", "exec", SevCritical, "递归文件删除"},
	{"shutil.rmtree", "exec", SevCritical, "递归目录删除"},
	{"child_process", "exec", SevCritical, "Node.js 子进程"},
	{"powershell", "exec", SevCritical, "PowerShell 执行"},
	{"cmd.exe", "exec", SevCritical, "Windows 命令行"},
	{"/bin/sh", "exec", SevWarning, "Shell 执行"},
	{"/bin/bash", "exec", SevWarning, "Bash 执行"},

	// --- Base64 obfuscation (common in malware) ---
	{"base64.b64decode", "obfuscation", SevWarning, "Base64 解码 (可能混淆)"},
	{"atob(", "obfuscation", SevWarning, "Base64 解码 (JS)"},
	{"fromcharcode", "obfuscation", SevWarning, "字符编码混淆"},
}

// StaticScan performs Layer 1 static analysis on skill content.
func (a *Auditor) StaticScan(content []byte) []Finding {
	var findings []Finding
	lower := strings.ToLower(string(content))
	lines := strings.Split(string(content), "\n")

	for _, pat := range staticPatterns {
		if strings.Contains(lower, pat.pattern) {
			// Find approximate line number
			lineNum := 0
			for i, line := range lines {
				if strings.Contains(strings.ToLower(line), pat.pattern) {
					lineNum = i + 1
					break
				}
			}
			findings = append(findings, Finding{
				Layer:    "static",
				Severity: pat.severity,
				Rule:     pat.category + "/" + pat.pattern,
				Detail:   pat.reason,
				Line:     lineNum,
			})
		}
	}

	// Check content size (excessively large skills are suspicious)
	if len(content) > 100_000 {
		findings = append(findings, Finding{
			Layer:    "static",
			Severity: SevWarning,
			Rule:     "size/too-large",
			Detail:   fmt.Sprintf("Skill 内容过大 (%d bytes)，可能隐藏恶意代码", len(content)),
		})
	}

	return findings
}

// ── Layer 2: Permission Audit ──

// AuditPermissions checks that declared permissions match actual content usage.
func (a *Auditor) AuditPermissions(skill *AdaptedSkill) []Finding {
	var findings []Finding
	lower := strings.ToLower(skill.Content)
	declared := make(map[string]bool)
	for _, p := range skill.Permissions {
		declared[strings.ToLower(p)] = true
	}

	// Check: content uses network but doesn't declare it
	networkIndicators := []string{"http://", "https://", "curl ", "wget ", "requests.", "fetch(", "net.dial", "socket."}
	for _, ind := range networkIndicators {
		if strings.Contains(lower, ind) {
			if !declared["network"] && !declared["http"] && !declared["net"] {
				findings = append(findings, Finding{
					Layer:    "permission",
					Severity: SevCritical,
					Rule:     "undeclared/network",
					Detail:   fmt.Sprintf("使用了网络功能 (%s) 但未声明 network 权限", ind),
				})
				break
			}
		}
	}

	// Check: content uses filesystem but doesn't declare it
	fsIndicators := []string{"open(", "readfile", "writefile", "os.read", "os.write", "fs.", "fopen"}
	for _, ind := range fsIndicators {
		if strings.Contains(lower, ind) {
			if !declared["write"] && !declared["filesystem"] && !declared["read-only"] && !declared["read"] {
				findings = append(findings, Finding{
					Layer:    "permission",
					Severity: SevWarning,
					Rule:     "undeclared/filesystem",
					Detail:   fmt.Sprintf("使用了文件操作 (%s) 但未声明 filesystem 权限", ind),
				})
				break
			}
		}
	}

	// Check: content uses shell/exec but doesn't declare it
	shellIndicators := []string{"subprocess", "os.system", "exec(", "child_process", "cmd.exe", "powershell", "/bin/sh"}
	for _, ind := range shellIndicators {
		if strings.Contains(lower, ind) {
			if !declared["shell"] && !declared["exec"] && !declared["command"] {
				findings = append(findings, Finding{
					Layer:    "permission",
					Severity: SevCritical,
					Rule:     "undeclared/shell",
					Detail:   fmt.Sprintf("使用了 shell/命令执行 (%s) 但未声明 shell 权限", ind),
				})
				break
			}
		}
	}

	// Check: no permissions declared at all (suspicious)
	if len(skill.Permissions) == 0 {
		findings = append(findings, Finding{
			Layer:    "permission",
			Severity: SevWarning,
			Rule:     "missing/permissions",
			Detail:   "Skill 未声明任何权限——应显式声明所需权限",
		})
	}

	return findings
}

// ── Layer 3: Sandbox Test ──

// SandboxTest performs a simulated execution in an isolated environment.
// For now, this performs additional heuristic analysis (full sandbox = future work).
func (a *Auditor) SandboxTest(ctx context.Context, skill *AdaptedSkill) []Finding {
	var findings []Finding

	// Timeout context for sandbox analysis
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Heuristic: check for encoded/obfuscated payloads
	content := skill.Content
	if countEncodedBlocks(content) > 3 {
		findings = append(findings, Finding{
			Layer:    "sandbox",
			Severity: SevWarning,
			Rule:     "obfuscation/encoded-blocks",
			Detail:   "包含多个编码块，可能用于混淆恶意代码",
		})
	}

	// Heuristic: excessive URL count
	urlCount := strings.Count(strings.ToLower(content), "http://") + strings.Count(strings.ToLower(content), "https://")
	if urlCount > 10 {
		findings = append(findings, Finding{
			Layer:    "sandbox",
			Severity: SevWarning,
			Rule:     "network/excessive-urls",
			Detail:   fmt.Sprintf("包含 %d 个 URL，外部依赖过多", urlCount),
		})
	}

	// Heuristic: content references system-critical paths
	criticalPaths := []string{"/etc/passwd", "/etc/shadow", "/windows/system32", "c:\\windows\\system32",
		"hkey_local_machine", "hkey_current_user"}
	for _, cp := range criticalPaths {
		if strings.Contains(strings.ToLower(content), cp) {
			findings = append(findings, Finding{
				Layer:    "sandbox",
				Severity: SevCritical,
				Rule:     "sandbox/critical-path",
				Detail:   fmt.Sprintf("引用了系统关键路径: %s", cp),
			})
		}
	}

	// Heuristic: self-referencing (skill tries to modify itself)
	if strings.Contains(strings.ToLower(content), "skill.md") && strings.Contains(strings.ToLower(content), "write") {
		findings = append(findings, Finding{
			Layer:    "sandbox",
			Severity: SevWarning,
			Rule:     "sandbox/self-modify",
			Detail:   "Skill 可能尝试自我修改",
		})
	}

	select {
	case <-ctx.Done():
		findings = append(findings, Finding{
			Layer:    "sandbox",
			Severity: SevCritical,
			Rule:     "sandbox/timeout",
			Detail:   "沙箱分析超时 (10s)，可能包含计算密集型操作",
		})
	default:
	}

	return findings
}

// ── Scoring ──

func (a *Auditor) scoreStatic(findings []Finding) int {
	score := 40 // start at max
	for _, f := range findings {
		if f.Layer != "static" {
			continue
		}
		switch f.Severity {
		case SevCritical:
			score -= 15
		case SevWarning:
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func (a *Auditor) scorePerm(findings []Finding) int {
	score := 30
	for _, f := range findings {
		if f.Layer != "permission" {
			continue
		}
		switch f.Severity {
		case SevCritical:
			score -= 15
		case SevWarning:
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func (a *Auditor) scoreSandbox(findings []Finding) int {
	score := 30
	for _, f := range findings {
		if f.Layer != "sandbox" {
			continue
		}
		switch f.Severity {
		case SevCritical:
			score -= 15
		case SevWarning:
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// ── Persistence ──

func (a *Auditor) saveReport(slug string, report *AuditReport) {
	dir := filepath.Join(a.dataDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("auditor: mkdir failed", "err", err)
		return
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	path := filepath.Join(dir, "audit_report.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Warn("auditor: write report failed", "err", err)
	}
}

// LoadReport reads a previously saved audit report for a skill.
func (a *Auditor) LoadReport(slug string) (*AuditReport, error) {
	path := filepath.Join(a.dataDir, slug, "audit_report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report AuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// ── Runtime monitoring ──

// RuntimeEvent records a single skill invocation's behavior.
type RuntimeEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Slug      string    `json:"slug"`
	Action    string    `json:"action"`  // e.g. "execute", "network", "file_write"
	Detail    string    `json:"detail"`
	Blocked   bool      `json:"blocked"` // true if behavior was blocked
}

// RecordRuntime appends a runtime event to the skill's runtime log.
func (a *Auditor) RecordRuntime(slug string, event RuntimeEvent) {
	if a.dataDir == "" {
		return
	}
	dir := filepath.Join(a.dataDir, slug)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "runtime_log.json")

	// Load existing events
	var events []RuntimeEvent
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &events)
	}

	// Keep last 1000 events
	events = append(events, event)
	if len(events) > 1000 {
		events = events[len(events)-1000:]
	}

	data, _ := json.MarshalIndent(events, "", "  ")
	os.WriteFile(path, data, 0644)
}

// ── Helpers ──

func countEncodedBlocks(s string) int {
	count := 0
	// Count base64-like blocks (long runs of alphanumeric + /+= chars)
	inBlock := false
	blockLen := 0
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			if !inBlock {
				inBlock = true
				blockLen = 0
			}
			blockLen++
		} else {
			if inBlock && blockLen > 64 {
				count++
			}
			inBlock = false
			blockLen = 0
		}
	}
	if inBlock && blockLen > 64 {
		count++
	}
	return count
}
