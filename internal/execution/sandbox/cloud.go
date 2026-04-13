package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CloudConfig configures the cloud sandbox runtime (E2B-compatible API).
type CloudConfig struct {
	Enabled  bool          `json:"enabled"`
	APIKey   string        `json:"api_key"`
	BaseURL  string        `json:"base_url"`  // platform API (default: https://api.e2b.app)
	Template string        `json:"template"`  // sandbox template ID (default: base)
	Timeout  time.Duration `json:"timeout"`
}

// DesktopSandbox holds state for a persistent E2B Desktop sandbox with VNC streaming.
type DesktopSandbox struct {
	ID          string   `json:"id"`
	StreamURL   string   `json:"stream_url"`
	AccessToken string   `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	VNCLog      []string `json:"vnc_log,omitempty"`
}

// DefaultCloudConfig returns defaults for E2B cloud sandbox.
func DefaultCloudConfig() CloudConfig {
	return CloudConfig{
		BaseURL:  "https://api.e2b.app",
		Template: "base",
		Timeout:  60 * time.Second,
	}
}

// CloudRunner executes code in a remote cloud sandbox via REST API.
// Supports E2B (api.e2b.app) and compatible providers.
type CloudRunner struct {
	cfg    CloudConfig
	client *http.Client
}

// NewCloudRunner creates a cloud-based Runner.
func NewCloudRunner(cfg CloudConfig) (*CloudRunner, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("cloud sandbox: API key required (set SANDBOX_CLOUD_API_KEY)")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.e2b.app"
	}
	if cfg.Template == "" {
		cfg.Template = "base"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &CloudRunner{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout + 10*time.Second},
	}, nil
}

func (r *CloudRunner) Type() string { return "cloud" }
func (r *CloudRunner) Close() error { return nil }

// CreateDesktop creates a persistent E2B Desktop sandbox with noVNC streaming.
// After creating the sandbox, starts x11vnc + novnc_proxy inside it so that the
// VNC stream URL is reachable from outside.
func (r *CloudRunner) CreateDesktop(ctx context.Context) (*DesktopSandbox, error) {
	template := r.cfg.Template
	if template == "" || template == "base" {
		template = "desktop"
	}

	body := map[string]any{"templateID": template}
	body["timeout"] = 1800 // 30 minutes — desktop sandbox needs long lifetime

	data, err := r.platformAPI(ctx, "POST", "/sandboxes", body)
	if err != nil {
		return nil, fmt.Errorf("create desktop sandbox: %w", err)
	}

	var resp struct {
		SandboxID       string  `json:"sandboxID"`
		ClientID        string  `json:"clientID"`
		EnvdAccessToken *string `json:"envdAccessToken"`
		TrafficToken    *string `json:"trafficAccessToken"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create response: %w (raw: %s)", err, string(data))
	}
	if resp.SandboxID == "" {
		return nil, fmt.Errorf("empty sandbox ID: %s", string(data))
	}

	sb := &cloudSandbox{ID: resp.SandboxID}
	if resp.EnvdAccessToken != nil {
		sb.AccessToken = *resp.EnvdAccessToken
	}

	vncLog := r.startDesktopVNC(ctx, sb)

	portBase := r.sandboxPortURL(sb, 6080)
	streamURL := portBase + "/vnc.html?autoconnect=true&resize=scale"

	return &DesktopSandbox{
		ID:          resp.SandboxID,
		StreamURL:   streamURL,
		AccessToken: sb.AccessToken,
		CreatedAt:   time.Now(),
		VNCLog:      vncLog,
	}, nil
}

// startDesktopVNC starts x11vnc and novnc_proxy inside the sandbox.
// All daemon commands are wrapped with `bash -c 'nohup ... & sleep 0.5 && echo ok'`
// because envd streams process output until exit; daemons never exit, causing timeout.
func (r *CloudRunner) startDesktopVNC(ctx context.Context, sb *cloudSandbox) []string {
	var log []string

	envdPort := r.findEnvdPort(ctx, sb)
	log = append(log, fmt.Sprintf("envd_port=%d", envdPort))

	time.Sleep(3 * time.Second)

	// Diagnostics: check existing display and installed binaries
	diagCmd := "echo DISPLAY=$DISPLAY && ls -la /tmp/.X*-lock 2>/dev/null || echo 'no X lock' && which Xvfb x11vnc 2>/dev/null && ps aux | grep -E 'Xvfb|x11vnc|vnc' | grep -v grep || echo 'no vnc procs'"
	diagResult := r.execInDesktop(ctx, sb, envdPort, diagCmd)
	log = append(log, fmt.Sprintf("diag: %s", diagResult))

	// Step 1: Start Xvfb — wrap as daemon so envd doesn't block
	xvfbCmd := `nohup Xvfb :0 -screen 0 1024x768x24 -ac > /tmp/xvfb.log 2>&1 & sleep 1 && echo "xvfb_pid=$!"`
	xvfbResult := r.execInDesktop(ctx, sb, envdPort, xvfbCmd)
	log = append(log, fmt.Sprintf("xvfb: %s", xvfbResult))

	time.Sleep(2 * time.Second)

	// Step 2: Start XFCE desktop — also backgrounded
	xfceCmd := `export DISPLAY=:0 && nohup startxfce4 > /tmp/xfce.log 2>&1 & sleep 1 && echo "xfce_pid=$!"`
	xfceResult := r.execInDesktop(ctx, sb, envdPort, xfceCmd)
	log = append(log, fmt.Sprintf("xfce: %s", xfceResult))

	time.Sleep(3 * time.Second)

	// Step 3: Start x11vnc (already supports -bg)
	vncCmd := "export DISPLAY=:0 && x11vnc -bg -display :0 -forever -shared -rfbport 5900 -nopw 2>/tmp/x11vnc_stderr.log"
	vncResult := r.execInDesktop(ctx, sb, envdPort, vncCmd)
	log = append(log, fmt.Sprintf("x11vnc: %s", vncResult))

	time.Sleep(1 * time.Second)

	// Step 4: Start noVNC proxy — backgrounded with nohup
	noVNCCmd := `nohup /opt/noVNC/utils/novnc_proxy --vnc localhost:5900 --listen 6080 --web /opt/noVNC > /tmp/novnc.log 2>&1 & sleep 1 && echo "novnc_pid=$!"`
	noVNCResult := r.execInDesktop(ctx, sb, envdPort, noVNCCmd)
	log = append(log, fmt.Sprintf("novnc: %s", noVNCResult))

	time.Sleep(2 * time.Second)

	// Final check: verify ports are listening
	checkCmd := `ss -tlnp 2>/dev/null | grep -E '5900|6080' || netstat -tlnp 2>/dev/null | grep -E '5900|6080' || echo "no_listeners"`
	checkResult := r.execInDesktop(ctx, sb, envdPort, checkCmd)
	log = append(log, fmt.Sprintf("ports: %s", checkResult))

	return log
}

// findEnvdPort returns the envd daemon port (49983 per E2B SDK).
func (r *CloudRunner) findEnvdPort(_ context.Context, _ *cloudSandbox) int {
	return 49983
}

// execInDesktop runs a bash command inside the desktop sandbox.
// When proxied (Tori), routes envd calls through Tori's envd proxy endpoint.
func (r *CloudRunner) execInDesktop(ctx context.Context, sb *cloudSandbox, envdPort int, command string) string {
	envdBase := r.sandboxEnvdURL(sb, envdPort)
	url := envdBase + "/process.Process/Start"
	if r.isProxied() {
		url += fmt.Sprintf("?port=%d", envdPort)
	}

	reqBody := map[string]any{
		"process": map[string]any{
			"cmd":  "bash",
			"args": []string{"-c", command},
		},
	}
	data, _ := json.Marshal(reqBody)

	shortCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(shortCtx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Sprintf("build_err: %v", err)
	}
	enveloped := connectEnvelope(data)
	req.Body = io.NopCloser(bytes.NewReader(enveloped))
	req.ContentLength = int64(len(enveloped))
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("X-API-Key", r.cfg.APIKey)
	if sb.AccessToken != "" {
		req.Header.Set("X-Access-Token", sb.AccessToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Sprintf("envd_err(%v)", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("envd_%d(%s)", resp.StatusCode, string(body))
	}
	return fmt.Sprintf("ok(%d) %s", resp.StatusCode, string(body))
}

// connectEnvelope wraps JSON data in Connect protocol envelope framing.
// Format: [1 byte flags][4 bytes big-endian length][JSON payload]
func connectEnvelope(data []byte) []byte {
	envelope := make([]byte, 5+len(data))
	envelope[0] = 0 // flags: no compression
	envelope[1] = byte(len(data) >> 24)
	envelope[2] = byte(len(data) >> 16)
	envelope[3] = byte(len(data) >> 8)
	envelope[4] = byte(len(data))
	copy(envelope[5:], data)
	return envelope
}

// ExecInDesktopSandbox runs a command in an existing desktop sandbox and returns the result.
func (r *CloudRunner) ExecInDesktopSandbox(ctx context.Context, sandboxID, accessToken, command string) (*RunResult, error) {
	sb := &cloudSandbox{ID: sandboxID, AccessToken: accessToken}
	envdPort := 49983

	start := time.Now()
	reqBody := map[string]any{
		"process": map[string]any{
			"cmd":  "bash",
			"args": []string{"-c", command},
		},
	}

	envdBase := r.sandboxEnvdURL(sb, envdPort)
	url := envdBase + "/process.Process/Start"
	if r.isProxied() {
		url += fmt.Sprintf("?port=%d", envdPort)
	}

	data, _ := json.Marshal(reqBody)
	enveloped := connectEnvelope(data)

	shortCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(shortCtx, "POST", url, bytes.NewReader(enveloped))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("X-API-Key", r.cfg.APIKey)
	if sb.AccessToken != "" {
		req.Header.Set("X-Access-Token", sb.AccessToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return &RunResult{ExitCode: -1, Stderr: err.Error(), Duration: time.Since(start)}, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &RunResult{ExitCode: -1, Stderr: fmt.Sprintf("envd %d: %s", resp.StatusCode, string(body)), Duration: time.Since(start)}, nil
	}
	return r.parseProcessResponse(body, start)
}

// WriteFileInDesktop writes a file inside an existing desktop sandbox.
func (r *CloudRunner) WriteFileInDesktop(ctx context.Context, sandboxID, accessToken, path, content string) error {
	sb := &cloudSandbox{ID: sandboxID, AccessToken: accessToken}
	return r.writeFile(ctx, sb, path, content)
}

// DestroyDesktop destroys a desktop sandbox.
func (r *CloudRunner) DestroyDesktop(ctx context.Context, sandboxID string) error {
	_, err := r.platformAPI(ctx, "DELETE", "/sandboxes/"+sandboxID, nil)
	return err
}

// DesktopStatus checks if a desktop sandbox is alive.
func (r *CloudRunner) DesktopStatus(ctx context.Context, sandboxID string) (map[string]any, error) {
	data, err := r.platformAPI(ctx, "GET", "/sandboxes/"+sandboxID, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result, nil
}

func (r *CloudRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	sb, err := r.createSandbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("cloud sandbox create: %w", err)
	}
	defer r.destroySandbox(context.Background(), sb.ID)

	for name, content := range req.Files {
		if err := r.writeFile(ctx, sb, "/home/user/"+name, content); err != nil {
			return nil, fmt.Errorf("cloud sandbox write %s: %w", name, err)
		}
	}

	if req.Code != "" {
		return r.runCode(ctx, sb, req)
	}
	if req.Command != "" {
		cmd := req.Command
		args := req.Args
		return r.exec(ctx, sb, cmd, args)
	}
	return nil, fmt.Errorf("either Code or Command must be specified")
}

func (r *CloudRunner) runCode(ctx context.Context, sb *cloudSandbox, req RunRequest) (*RunResult, error) {
	lr, ok := defaultLangRunners[req.Language]
	if !ok {
		return &RunResult{ExitCode: -1, Stderr: fmt.Sprintf("unsupported language: %s", req.Language)}, nil
	}

	filename := "/home/user/main" + lr.Ext
	if err := r.writeFile(ctx, sb, filename, req.Code); err != nil {
		return nil, fmt.Errorf("cloud sandbox write code: %w", err)
	}

	cmd := lr.Cmd
	args := []string{filename}
	if req.Language == "go" {
		args = []string{"run", filename}
	}
	return r.exec(ctx, sb, cmd, args)
}

// --- Cloud sandbox state ---

type cloudSandbox struct {
	ID          string
	AccessToken string // envdAccessToken for sandbox-level auth
}

// --- E2B Platform API (api.e2b.app) ---

type e2bCreateResp struct {
	SandboxID      string `json:"sandboxID"`
	EnvdAccessToken *string `json:"envdAccessToken"`
}

func (r *CloudRunner) createSandbox(ctx context.Context) (*cloudSandbox, error) {
	body := map[string]any{"templateID": r.cfg.Template}
	if r.cfg.Timeout > 0 {
		body["timeout"] = int(r.cfg.Timeout.Seconds())
	}

	data, err := r.platformAPI(ctx, "POST", "/sandboxes", body)
	if err != nil {
		return nil, err
	}
	var resp e2bCreateResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}
	if resp.SandboxID == "" {
		return nil, fmt.Errorf("empty sandbox ID in response: %s", string(data))
	}
	sb := &cloudSandbox{ID: resp.SandboxID}
	if resp.EnvdAccessToken != nil {
		sb.AccessToken = *resp.EnvdAccessToken
	}
	return sb, nil
}

func (r *CloudRunner) destroySandbox(ctx context.Context, sbID string) {
	_, _ = r.platformAPI(ctx, "DELETE", "/sandboxes/"+sbID, nil)
}

// --- Sandbox Envd API (49983-{sandboxID}.e2b.app) ---
// Uses Connect protocol for process execution.

// sandboxEnvdURL returns the envd base URL for the given sandbox.
func (r *CloudRunner) sandboxEnvdURL(sb *cloudSandbox, port int) string {
	if r.isProxied() {
		base := strings.TrimRight(r.cfg.BaseURL, "/")
		return fmt.Sprintf("%s/sandboxes/%s/envd", base, sb.ID)
	}
	return fmt.Sprintf("https://%d-%s.e2b.app", port, sb.ID)
}

// sandboxPortURL returns the public browser-accessible URL for a sandbox port.
// Always uses E2B domain directly — this URL is for the user's browser, not API calls.
func (r *CloudRunner) sandboxPortURL(sb *cloudSandbox, port int) string {
	return fmt.Sprintf("https://%d-%s.e2b.app", port, sb.ID)
}

// isProxied returns true when BaseURL is not the E2B platform API,
// meaning a proxy (like ToriAPI) handles both platform and envd routing.
func (r *CloudRunner) isProxied() bool {
	return !strings.Contains(r.cfg.BaseURL, "e2b.app") &&
		!strings.Contains(r.cfg.BaseURL, "e2b.dev")
}

func (r *CloudRunner) exec(ctx context.Context, sb *cloudSandbox, cmd string, args []string) (*RunResult, error) {
	start := time.Now()

	reqBody := map[string]any{
		"process": map[string]any{
			"cmd":  cmd,
			"args": args,
		},
	}

	envdURL := r.sandboxEnvdURL(sb, 49983)
	data, err := r.envdAPI(ctx, sb, envdURL+"/process.Process/Start", reqBody)
	if err != nil {
		if r.isProxied() {
			return r.execSimple(ctx, sb, cmd, args, start)
		}
		return &RunResult{ExitCode: -1, Stderr: err.Error(), Duration: time.Since(start)}, nil
	}

	return r.parseProcessResponse(data, start)
}

// execSimple tries a simpler REST endpoint for non-E2B providers.
func (r *CloudRunner) execSimple(ctx context.Context, sb *cloudSandbox, cmd string, args []string, start time.Time) (*RunResult, error) {
	fullCmd := cmd
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	body := map[string]any{"cmd": fullCmd}
	if r.cfg.Timeout > 0 {
		body["timeout"] = int(r.cfg.Timeout.Seconds())
	}

	data, err := r.platformAPI(ctx, "POST", "/sandboxes/"+sb.ID+"/commands", body)
	if err != nil {
		return &RunResult{ExitCode: -1, Stderr: err.Error(), Duration: time.Since(start)}, nil
	}

	var resp struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode *int   `json:"exitCode"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return &RunResult{ExitCode: -1, Stderr: "parse response: " + err.Error(), Duration: time.Since(start)}, nil
	}
	if resp.Error != "" {
		return &RunResult{ExitCode: -1, Stderr: resp.Error, Duration: time.Since(start)}, nil
	}
	exitCode := 0
	if resp.ExitCode != nil {
		exitCode = *resp.ExitCode
	}
	return &RunResult{ExitCode: exitCode, Stdout: resp.Stdout, Stderr: resp.Stderr, Duration: time.Since(start)}, nil
}

// parseProcessResponse parses E2B Connect protocol streaming response.
func (r *CloudRunner) parseProcessResponse(data []byte, start time.Time) (*RunResult, error) {
	result := &RunResult{Duration: time.Since(start)}

	// Connect protocol returns newline-delimited JSON events
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var wrapper struct {
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(line, &wrapper); err != nil {
			continue
		}
		raw := wrapper.Result
		if len(raw) == 0 {
			raw = line
		}

		var evt struct {
			Event struct {
				Data *struct {
					Stdout *string `json:"stdout"`
					Stderr *string `json:"stderr"`
				} `json:"data"`
				End *struct {
					ExitCode int    `json:"exit_code"`
					Status   string `json:"status"`
					Exited   bool   `json:"exited"`
				} `json:"end"`
			} `json:"event"`
		}
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}
		if evt.Event.Data != nil {
			if evt.Event.Data.Stdout != nil {
				result.Stdout += *evt.Event.Data.Stdout
			}
			if evt.Event.Data.Stderr != nil {
				result.Stderr += *evt.Event.Data.Stderr
			}
		}
		if evt.Event.End != nil {
			result.ExitCode = evt.Event.End.ExitCode
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// --- File operations ---

func (r *CloudRunner) writeFile(ctx context.Context, sb *cloudSandbox, path, content string) error {
	if r.isProxied() {
		body := map[string]any{"path": path, "content": content}
		_, err := r.platformAPI(ctx, "POST", "/sandboxes/"+sb.ID+"/files", body)
		return err
	}

	envdURL := r.sandboxEnvdURL(sb, 49983)
	fileURL := envdURL + "/files?path=" + path
	req, err := http.NewRequestWithContext(ctx, "POST", fileURL, strings.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if sb.AccessToken != "" {
		req.Header.Set("X-Access-Token", sb.AccessToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("write file: %d — %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// --- HTTP helpers ---

// platformAPI calls the E2B Platform API (api.e2b.app).
func (r *CloudRunner) platformAPI(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	url := strings.TrimRight(r.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", r.cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %s %s: %d — %s", method, path, resp.StatusCode, string(data))
	}
	return data, nil
}

// envdAPI calls the sandbox envd API using Connect protocol with envelope framing.
func (r *CloudRunner) envdAPI(ctx context.Context, sb *cloudSandbox, url string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	enveloped := connectEnvelope(data)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(enveloped))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("X-API-Key", r.cfg.APIKey)
	if sb.AccessToken != "" {
		req.Header.Set("X-Access-Token", sb.AccessToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("envd: %d — %s", resp.StatusCode, string(respData))
	}
	return respData, nil
}
