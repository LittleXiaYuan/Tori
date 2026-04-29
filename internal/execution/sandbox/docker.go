package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// DockerRuntime — Docker sandbox with container pool,
// image management, and security hardening.
// ──────────────────────────────────────────────

// DockerRuntime manages Docker-based sandbox execution.
type DockerRuntime struct {
	cfg  DockerConfig
	pool *containerPool

	mu     sync.Mutex
	closed bool
}

// containerInfo tracks a pool container.
type containerInfo struct {
	ID        string
	Image     string
	CreatedAt time.Time
	InUse     bool
}

// containerPool manages pre-warmed Docker containers.
type containerPool struct {
	mu         sync.Mutex
	containers map[string]*containerInfo // containerID -> info
	available  map[string][]string       // image -> available containerIDs
	cfg        DockerConfig
	stopCh     chan struct{}
}

// ──────────────────────────────────────────────
// DockerRuntime creation
// ──────────────────────────────────────────────

// NewDockerRuntime creates a Docker sandbox runtime.
// Returns error if Docker is not available.
func NewDockerRuntime(cfg DockerConfig) (*DockerRuntime, error) {
	if !isDockerAvailable() {
		return nil, fmt.Errorf("docker is not available")
	}

	if cfg.BaseDir == "" {
		cfg.BaseDir = DefaultDockerConfig().BaseDir
	}

	dr := &DockerRuntime{
		cfg: cfg,
		pool: &containerPool{
			containers: make(map[string]*containerInfo),
			available:  make(map[string][]string),
			cfg:        cfg,
			stopCh:     make(chan struct{}),
		},
	}

	// Start idle container cleanup goroutine
	go dr.pool.cleanupLoop()

	// Pre-warm containers for configured images
	if cfg.PoolSize > 0 {
		go dr.prewarm()
	}

	slog.Info("docker sandbox runtime initialized",
		"pool_size", cfg.PoolSize,
		"max_containers", cfg.MaxContainers,
		"memory", cfg.MemoryLimit,
		"readonly_rootfs", cfg.ReadOnlyRootfs,
	)
	return dr, nil
}

func (dr *DockerRuntime) Type() string { return "docker" }

// Close shuts down the Docker runtime, stopping and removing all pool containers.
func (dr *DockerRuntime) Close() error {
	dr.mu.Lock()
	if dr.closed {
		dr.mu.Unlock()
		return nil
	}
	dr.closed = true
	dr.mu.Unlock()

	close(dr.pool.stopCh)
	dr.pool.destroyAll()
	return nil
}

// ──────────────────────────────────────────────
// Run — execute code/command in Docker container
// ──────────────────────────────────────────────

func (dr *DockerRuntime) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	dr.mu.Lock()
	if dr.closed {
		dr.mu.Unlock()
		return nil, fmt.Errorf("docker runtime is closed")
	}
	dr.mu.Unlock()

	image := dr.resolveImage(req.Language)

	// Validate image against whitelist
	if !dr.isImageAllowed(image) {
		return &RunResult{ExitCode: -1, Stderr: fmt.Sprintf("image not allowed: %s", image)}, nil
	}

	// Ensure image is available locally
	if err := dr.ensureImage(ctx, image); err != nil {
		return nil, fmt.Errorf("pull image %s: %w", image, err)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = dr.cfg.Timeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Try to get a warm container from pool
	containerID := dr.pool.checkout(image)
	if containerID != "" {
		result, err := dr.execInContainer(ctx, containerID, req)
		dr.pool.release(containerID) // release back for cleanup + re-creation
		if result != nil {
			result.Duration = time.Since(start)
		}
		if err == nil {
			return result, nil
		}
		// Warm container failed — fall through to cold start
		slog.Warn("warm container exec failed, falling back to cold start", "err", err)
	}

	// Cold start: create one-shot container
	return dr.runColdStart(ctx, start, image, req)
}

// ──────────────────────────────────────────────
// Image management
// ──────────────────────────────────────────────

func (dr *DockerRuntime) resolveImage(language string) string {
	if language != "" && dr.cfg.LanguageImages != nil {
		if img, ok := dr.cfg.LanguageImages[language]; ok {
			return img
		}
	}
	if dr.cfg.DefaultImage != "" {
		return dr.cfg.DefaultImage
	}
	return "python:3.12-slim"
}

func (dr *DockerRuntime) isImageAllowed(image string) bool {
	// If no whitelist, allow all images from LanguageImages + DefaultImage
	if len(dr.cfg.AllowedImages) == 0 {
		// Check against known images
		if image == dr.cfg.DefaultImage {
			return true
		}
		for _, img := range dr.cfg.LanguageImages {
			if img == image {
				return true
			}
		}
		return false
	}
	for _, allowed := range dr.cfg.AllowedImages {
		if allowed == image {
			return true
		}
	}
	return false
}

func (dr *DockerRuntime) ensureImage(ctx context.Context, image string) error {
	// Check if image exists locally first
	check := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if check.Run() == nil {
		return nil // already available
	}
	// Pull the image
	slog.Info("docker: pulling image", "image", image)
	pull := exec.CommandContext(ctx, "docker", "pull", image)
	var stderr bytes.Buffer
	pull.Stderr = &stderr
	if err := pull.Run(); err != nil {
		return fmt.Errorf("docker pull %s: %s", image, stderr.String())
	}
	return nil
}

// ListImages returns the set of configured Docker images.
func (dr *DockerRuntime) ListImages() map[string]string {
	result := make(map[string]string)
	for lang, img := range dr.cfg.LanguageImages {
		result[lang] = img
	}
	return result
}

// ──────────────────────────────────────────────
// Cold start execution (docker run --rm)
// ──────────────────────────────────────────────

func (dr *DockerRuntime) runColdStart(ctx context.Context, start time.Time, image string, req RunRequest) (*RunResult, error) {
	args := dr.buildDockerRunArgs(image, req)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	err := cmd.Run()
	duration := time.Since(start)

	result := &RunResult{
		Stdout:   truncate(stdout.String(), dr.cfg.MaxOutputBytes),
		Stderr:   truncate(stderr.String(), dr.cfg.MaxOutputBytes),
		Duration: duration,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if ctx.Err() == context.DeadlineExceeded {
				result.Stderr = "execution timeout exceeded"
			} else {
				result.Stderr = err.Error()
			}
		}
	}
	return result, nil
}

func (dr *DockerRuntime) buildDockerRunArgs(image string, req RunRequest) []string {
	args := []string{"run", "--rm"}

	// Security hardening
	args = append(args, "--security-opt=no-new-privileges")

	if dr.cfg.ReadOnlyRootfs {
		args = append(args, "--read-only")
		// Provide writable tmpfs for /tmp and /workspace
		args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m")
		args = append(args, "--tmpfs", "/workspace:rw,exec,nosuid,size=128m")
	}

	if dr.cfg.NonRootUser {
		args = append(args, "--user", "1000:1000")
	}

	// Resource limits
	if dr.cfg.MemoryLimit != "" {
		args = append(args, "--memory", dr.cfg.MemoryLimit)
		args = append(args, "--memory-swap", dr.cfg.MemoryLimit) // disable swap
	}
	if dr.cfg.CPULimit != "" {
		args = append(args, "--cpus", dr.cfg.CPULimit)
	}
	if dr.cfg.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", dr.cfg.PidsLimit))
	}

	// Network isolation
	if !dr.cfg.NetworkEnabled {
		args = append(args, "--network", "none")
	}

	// Environment variables
	for k, v := range req.Env {
		args = append(args, "-e", k+"="+v)
	}

	// Working directory
	args = append(args, "-w", "/workspace")

	// Image
	args = append(args, image)

	// Code mode: needs shell wrapper to write files and execute
	if req.Code != "" {
		cmdStr := dr.buildCodeCommand(req)
		args = append(args, "sh", "-c", cmdStr)
	} else if req.Command != "" {
		// Command mode: pass command and args directly
		args = append(args, req.Command)
		args = append(args, req.Args...)
	}

	return args
}

// buildCodeCommand constructs a shell command string for Code execution.
// It writes source files and additional files, then runs the appropriate interpreter.
func (dr *DockerRuntime) buildCodeCommand(req RunRequest) string {
	lr, ok := defaultLangRunners[req.Language]
	if !ok {
		return "echo 'unsupported language' >&2; exit 1"
	}
	filename := "main" + lr.Ext

	// Escape single quotes in code for shell safety
	escapedCode := strings.ReplaceAll(req.Code, "'", "'\\''")

	var cmd string
	// Write additional files (sanitize names to prevent shell injection)
	for name, content := range req.Files {
		safeName := filepath.Base(filepath.Clean(name))
		if safeName == "." || safeName == "/" || safeName == "" || strings.ContainsAny(safeName, "'\"$`\\;|&(){}") {
			continue
		}
		escapedContent := strings.ReplaceAll(content, "'", "'\\''")
		cmd += fmt.Sprintf("printf '%%s' '%s' > '/workspace/%s' && ", escapedContent, safeName)
	}
	cmd += fmt.Sprintf("printf '%%s' '%s' > /workspace/%s && ", escapedCode, filename)

	if req.Language == "go" {
		cmd += fmt.Sprintf("cd /workspace && %s run %s", lr.Cmd, filename)
	} else {
		cmd += fmt.Sprintf("%s /workspace/%s", lr.Cmd, filename)
	}
	return cmd
}

// buildExecCommand constructs a shell command string for docker exec.
// Used by the container pool path where we always need sh -c wrapping.
func (dr *DockerRuntime) buildExecCommand(req RunRequest) string {
	if req.Code != "" {
		return dr.buildCodeCommand(req)
	}
	if req.Command != "" {
		cmd := req.Command
		if len(req.Args) > 0 {
			cmd += " " + strings.Join(req.Args, " ")
		}
		return cmd
	}
	return ""
}

// ──────────────────────────────────────────────
// Warm container execution (docker exec)
// ──────────────────────────────────────────────

func (dr *DockerRuntime) execInContainer(ctx context.Context, containerID string, req RunRequest) (*RunResult, error) {
	cmdStr := dr.buildExecCommand(req)
	if cmdStr == "" {
		return nil, fmt.Errorf("no command to execute")
	}

	execArgs := []string{"exec"}
	if dr.cfg.NonRootUser {
		execArgs = append(execArgs, "--user", "1000:1000")
	}
	for k, v := range req.Env {
		execArgs = append(execArgs, "-e", k+"="+v)
	}
	execArgs = append(execArgs, containerID, "sh", "-c", cmdStr)

	cmd := exec.CommandContext(ctx, "docker", execArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	err := cmd.Run()

	result := &RunResult{
		Stdout: truncate(stdout.String(), dr.cfg.MaxOutputBytes),
		Stderr: truncate(stderr.String(), dr.cfg.MaxOutputBytes),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
	}
	return result, nil
}

// ──────────────────────────────────────────────
// Container Pool
// ──────────────────────────────────────────────

func newContainerPool(cfg DockerConfig) *containerPool {
	return &containerPool{
		containers: make(map[string]*containerInfo),
		available:  make(map[string][]string),
		cfg:        cfg,
		stopCh:     make(chan struct{}),
	}
}

// create creates a new warm container for the given image.
func (p *containerPool) create(image string) (string, error) {
	p.mu.Lock()
	total := len(p.containers)
	p.mu.Unlock()

	if total >= p.cfg.MaxContainers {
		return "", fmt.Errorf("container pool full (%d/%d)", total, p.cfg.MaxContainers)
	}

	name := fmt.Sprintf("tori-sandbox-%s", uuid.New().String()[:8])

	args := []string{"run", "-d", "--name", name}

	// Security hardening (same as cold start)
	args = append(args, "--security-opt=no-new-privileges")
	if p.cfg.ReadOnlyRootfs {
		args = append(args, "--read-only")
		args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m")
		args = append(args, "--tmpfs", "/workspace:rw,exec,nosuid,size=128m")
	}
	if p.cfg.MemoryLimit != "" {
		args = append(args, "--memory", p.cfg.MemoryLimit)
		args = append(args, "--memory-swap", p.cfg.MemoryLimit)
	}
	if p.cfg.CPULimit != "" {
		args = append(args, "--cpus", p.cfg.CPULimit)
	}
	if p.cfg.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", p.cfg.PidsLimit))
	}
	if !p.cfg.NetworkEnabled {
		args = append(args, "--network", "none")
	}

	args = append(args, "-w", "/workspace")
	args = append(args, image, "sleep", "infinity")

	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("create container: %s", stderr.String())
	}

	containerID := strings.TrimSpace(string(out))
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	p.mu.Lock()
	p.containers[containerID] = &containerInfo{
		ID:        containerID,
		Image:     image,
		CreatedAt: time.Now(),
	}
	p.available[image] = append(p.available[image], containerID)
	p.mu.Unlock()

	slog.Debug("docker pool: container created", "id", containerID, "image", image)
	return containerID, nil
}

// checkout takes an available container for the given image.
func (p *containerPool) checkout(image string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	ids := p.available[image]
	if len(ids) == 0 {
		return ""
	}

	// Take the last one (LIFO for cache warmth)
	id := ids[len(ids)-1]
	p.available[image] = ids[:len(ids)-1]

	if ci, ok := p.containers[id]; ok {
		ci.InUse = true
	}
	return id
}

// release marks a container as no longer in use and schedules it for removal.
// Used containers are destroyed (not reused) for isolation.
func (p *containerPool) release(containerID string) {
	p.mu.Lock()
	ci, ok := p.containers[containerID]
	if ok {
		ci.InUse = false
		delete(p.containers, containerID)
		// Remove from available lists
		if ci.Image != "" {
			ids := p.available[ci.Image]
			for i, id := range ids {
				if id == containerID {
					p.available[ci.Image] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
	}
	p.mu.Unlock()

	// Destroy asynchronously
	go destroyContainer(containerID)

	// Replenish pool in background
	if ok && ci.Image != "" {
		go func(image string) {
			p.mu.Lock()
			count := len(p.available[image])
			p.mu.Unlock()
			if count < p.cfg.PoolSize {
				if _, err := p.create(image); err != nil {
					slog.Debug("docker pool: replenish failed", "image", image, "err", err)
				}
			}
		}(ci.Image)
	}
}

// cleanupLoop periodically removes idle containers that exceed IdleTimeout.
func (p *containerPool) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			p.cleanupIdle(now)
		}
	}
}

func (p *containerPool) cleanupIdle(now time.Time) {
	p.mu.Lock()
	var toRemove []string
	for id, ci := range p.containers {
		if !ci.InUse && now.Sub(ci.CreatedAt) > p.cfg.IdleTimeout {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		ci := p.containers[id]
		delete(p.containers, id)
		if ci != nil && ci.Image != "" {
			ids := p.available[ci.Image]
			for i, cid := range ids {
				if cid == id {
					p.available[ci.Image] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
	}
	p.mu.Unlock()

	for _, id := range toRemove {
		slog.Debug("docker pool: removing idle container", "id", id)
		destroyContainer(id)
	}
}

// destroyAll stops and removes all pool containers.
func (p *containerPool) destroyAll() {
	p.mu.Lock()
	ids := make([]string, 0, len(p.containers))
	for id := range p.containers {
		ids = append(ids, id)
	}
	p.containers = make(map[string]*containerInfo)
	p.available = make(map[string][]string)
	p.mu.Unlock()

	for _, id := range ids {
		destroyContainer(id)
	}
	slog.Info("docker pool: all containers destroyed", "count", len(ids))
}

// Stats returns pool statistics.
func (p *containerPool) stats() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(p.containers)
	inUse := 0
	avail := 0
	for _, ci := range p.containers {
		if ci.InUse {
			inUse++
		}
	}
	for _, ids := range p.available {
		avail += len(ids)
	}

	return map[string]any{
		"total":     total,
		"in_use":    inUse,
		"available": avail,
		"max":       p.cfg.MaxContainers,
	}
}

// ──────────────────────────────────────────────
// Pre-warm
// ──────────────────────────────────────────────

func (dr *DockerRuntime) prewarm() {
	// Collect unique images
	images := make(map[string]bool)
	for _, img := range dr.cfg.LanguageImages {
		images[img] = true
	}
	if dr.cfg.DefaultImage != "" {
		images[dr.cfg.DefaultImage] = true
	}

	for image := range images {
		// Ensure image is pulled
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := dr.ensureImage(ctx, image); err != nil {
			slog.Warn("docker pool: failed to pull image for pre-warm", "image", image, "err", err)
			cancel()
			continue
		}
		cancel()

		// Create pool containers
		for i := 0; i < dr.cfg.PoolSize; i++ {
			if _, err := dr.pool.create(image); err != nil {
				slog.Warn("docker pool: pre-warm failed", "image", image, "err", err)
				break
			}
		}
	}
	slog.Info("docker pool: pre-warm complete", "stats", dr.pool.stats())
}

// PoolStats returns container pool statistics.
func (dr *DockerRuntime) PoolStats() map[string]any {
	return dr.pool.stats()
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

func destroyContainer(containerID string) {
	// Force stop + remove
	stop := exec.Command("docker", "rm", "-f", containerID)
	_ = stop.Run()
}
