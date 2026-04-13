package general

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
)

type desktopSession struct {
	SandboxID   string
	AccessToken string
}

type ComputerUseSkill struct {
	cloudRunner *sandbox.CloudRunner
	mu          sync.RWMutex
	sessions    map[string]*desktopSession // key: tenantID
}

func NewComputerUseSkill() *ComputerUseSkill {
	return &ComputerUseSkill{
		sessions: make(map[string]*desktopSession),
	}
}

func (s *ComputerUseSkill) Name() string { return "computer_use" }
func (s *ComputerUseSkill) Description() string {
	return "创建和管理云端桌面沙箱（E2B Desktop），可执行代码、浏览网页、操作文件。返回 VNC 画面链接供用户查看。"
}
func (s *ComputerUseSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"create", "status", "destroy", "exec"},
				"description": "操作：create=创建桌面, status=查看状态, destroy=销毁, exec=在桌面沙箱内执行命令",
			},
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "沙箱ID（exec/destroy时必填，不填则使用最近创建的沙箱）",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "exec操作时要执行的shell命令",
			},
			"code": map[string]any{
				"type":        "string",
				"description": "exec操作时要执行的代码（与command二选一）",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "代码语言（python/javascript/go），与code参数配合使用",
			},
		},
		"required": []string{"action"},
	}
}

func (s *ComputerUseSkill) ensureRunner() error {
	if s.cloudRunner != nil {
		return nil
	}

	cfg := sandbox.CloudConfig{
		Enabled:  true,
		APIKey:   os.Getenv("SANDBOX_CLOUD_API_KEY"),
		BaseURL:  os.Getenv("SANDBOX_CLOUD_BASE_URL"),
		Template: "desktop",
		Timeout:  5 * time.Minute,
	}

	if cfg.APIKey == "" {
		if toriBase := strings.TrimSpace(os.Getenv("TORI_API_BASE_URL")); toriBase != "" {
			if llmKey := strings.TrimSpace(os.Getenv("LLM_API_KEY")); llmKey != "" {
				cfg.APIKey = llmKey
				trimmed := strings.TrimRight(toriBase, "/")
				if strings.HasSuffix(trimmed, "/v1") {
					cfg.BaseURL = trimmed
				} else {
					cfg.BaseURL = trimmed + "/v1"
				}
			}
		}
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("no sandbox API key configured (set SANDBOX_CLOUD_API_KEY or TORI_API_BASE_URL+LLM_API_KEY)")
	}

	cr, err := sandbox.NewCloudRunner(cfg)
	if err != nil {
		return fmt.Errorf("init cloud runner: %w", err)
	}
	s.cloudRunner = cr
	return nil
}

func (s *ComputerUseSkill) probeToriSandbox() bool {
	toriBase := strings.TrimSpace(os.Getenv("TORI_API_BASE_URL"))
	if toriBase == "" {
		toriBase = strings.TrimSpace(os.Getenv("SANDBOX_CLOUD_BASE_URL"))
	}
	if toriBase == "" {
		return false
	}

	trimmed := strings.TrimRight(toriBase, "/")
	probeURL := trimmed + "/sandboxes/status"
	if !strings.HasSuffix(trimmed, "/v1") {
		probeURL = trimmed + "/v1/sandboxes/status"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	var status struct {
		Configured bool `json:"configured"`
	}
	if json.Unmarshal(body, &status) != nil {
		return false
	}
	return status.Configured
}

func (s *ComputerUseSkill) getSession(tenantID string) *desktopSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[tenantID]
}

func (s *ComputerUseSkill) setSession(tenantID string, sess *desktopSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess == nil {
		delete(s.sessions, tenantID)
	} else {
		s.sessions[tenantID] = sess
	}
}

func (s *ComputerUseSkill) findTokenBySandboxID(sandboxID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.sessions {
		if sess.SandboxID == sandboxID {
			return sess.AccessToken
		}
	}
	return ""
}

func (s *ComputerUseSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	action, _ := args["action"].(string)
	sessionKey := env.TenantID + ":" + env.StudentID
	if sessionKey == ":" {
		sessionKey = "_default"
	}

	if action != "status" {
		if err := s.ensureRunner(); err != nil {
			return "", err
		}
	}

	switch action {
	case "create":
		if !s.probeToriSandbox() {
			slog.Warn("computer_use: Tori sandbox not configured, E2B may not work")
		}
		ds, err := s.cloudRunner.CreateDesktop(ctx)
		if err != nil {
			return "", fmt.Errorf("create desktop: %w", err)
		}
		s.setSession(sessionKey, &desktopSession{
			SandboxID:   ds.ID,
			AccessToken: ds.AccessToken,
		})
		result := map[string]any{
			"sandbox_id": ds.ID,
			"stream_url": ds.StreamURL,
			"created_at": ds.CreatedAt.Format(time.RFC3339),
			"message":    "桌面沙箱已创建。用户可通过 stream_url 查看桌面画面。",
		}
		out, _ := json.Marshal(result)
		return string(out), nil

	case "status":
		available := s.probeToriSandbox()
		sess := s.getSession(sessionKey)
		activeSbID := ""
		if sess != nil {
			activeSbID = sess.SandboxID
		}
		result := map[string]any{
			"sandbox_available":  available,
			"cloud_runner_ready": s.cloudRunner != nil,
			"active_sandbox_id":  activeSbID,
		}
		out, _ := json.Marshal(result)
		return string(out), nil

	case "destroy":
		sandboxID, _ := args["sandbox_id"].(string)
		sess := s.getSession(sessionKey)
		if sandboxID == "" && sess != nil {
			sandboxID = sess.SandboxID
		}
		if sandboxID == "" {
			return "", fmt.Errorf("sandbox_id is required for destroy")
		}
		err := s.cloudRunner.DestroyDesktop(ctx, sandboxID)
		if err != nil {
			return "", fmt.Errorf("destroy: %w", err)
		}
		if sess != nil && sandboxID == sess.SandboxID {
			s.setSession(sessionKey, nil)
		}
		return `{"ok": true, "message": "桌面沙箱已销毁"}`, nil

	case "exec":
		sandboxID, _ := args["sandbox_id"].(string)
		sess := s.getSession(sessionKey)
		accessToken := ""
		if sandboxID == "" && sess != nil {
			sandboxID = sess.SandboxID
			accessToken = sess.AccessToken
		} else if sandboxID != "" {
			if sess != nil && sess.SandboxID == sandboxID {
				accessToken = sess.AccessToken
			} else {
				accessToken = s.findTokenBySandboxID(sandboxID)
			}
		}

		command, _ := args["command"].(string)
		code, _ := args["code"].(string)
		lang, _ := args["language"].(string)

		if command == "" && code == "" {
			return "", fmt.Errorf("command or code is required for exec")
		}

		if sandboxID != "" {
			shellCmd := command
			if code != "" {
				if lang == "" {
					lang = "python"
				}
				ext := langExt(lang)
				tmpFile := fmt.Sprintf("/tmp/agent_exec_%d%s", time.Now().UnixNano(), ext)

				if err := s.cloudRunner.WriteFileInDesktop(ctx, sandboxID, accessToken, tmpFile, code); err != nil {
					return "", fmt.Errorf("write code file: %w", err)
				}
				shellCmd = langRunCmd(lang, tmpFile)
			}

			result, err := s.cloudRunner.ExecInDesktopSandbox(ctx, sandboxID, accessToken, shellCmd)
			if err != nil {
				return "", fmt.Errorf("exec in desktop: %w", err)
			}
			out, _ := json.Marshal(result)
			return string(out), nil
		}

		var req sandbox.RunRequest
		if code != "" {
			if lang == "" {
				lang = "python"
			}
			req = sandbox.RunRequest{Language: lang, Code: code}
		} else {
			req = sandbox.RunRequest{Command: command}
		}
		result, err := s.cloudRunner.Run(ctx, req)
		if err != nil {
			return "", fmt.Errorf("exec: %w", err)
		}
		out, _ := json.Marshal(result)
		return string(out), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use create/status/destroy/exec)", action)
	}
}

func langExt(lang string) string {
	switch lang {
	case "python":
		return ".py"
	case "javascript":
		return ".js"
	case "go":
		return ".go"
	case "shell", "bash":
		return ".sh"
	default:
		return ".py"
	}
}

func langRunCmd(lang, file string) string {
	switch lang {
	case "python":
		return "python3 " + file
	case "javascript":
		return "node " + file
	case "go":
		return "go run " + file
	case "shell", "bash":
		return "bash " + file
	default:
		return "python3 " + file
	}
}
