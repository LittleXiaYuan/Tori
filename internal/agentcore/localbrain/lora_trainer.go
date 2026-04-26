package localbrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TrainerMode controls which training backend to use.
type TrainerMode string

const (
	TrainerModeAuto   TrainerMode = "auto"
	TrainerModeRemote TrainerMode = "remote"
	TrainerModeLocal  TrainerMode = "local"
)

const maxStderrCapture = 4096

// TrainerConfig configures the LoRA training backend.
type TrainerConfig struct {
	Mode            TrainerMode
	RemoteURL       string
	RemoteKey       string
	ScriptPath      string
	Timeout         time.Duration
	PollInterval    time.Duration
	MaxSeqLength    int
	Seed            int
	TrustRemoteCode bool
	TargetModules   []string
}

// DefaultTrainerConfig returns sensible defaults.
func DefaultTrainerConfig() TrainerConfig {
	return TrainerConfig{
		Mode:         TrainerModeAuto,
		ScriptPath:   "scripts/lora_train.py",
		Timeout:      2 * time.Hour,
		PollInterval: 30 * time.Second,
		MaxSeqLength: 2048,
		Seed:         42,
	}
}

// TrainerConfigFromEnv builds a TrainerConfig from environment variables.
func TrainerConfigFromEnv() TrainerConfig {
	cfg := DefaultTrainerConfig()

	if mode := os.Getenv("LORA_TRAIN_MODE"); mode != "" {
		cfg.Mode = TrainerMode(strings.ToLower(mode))
	}
	if url := os.Getenv("LORA_TRAIN_API_URL"); url != "" {
		cfg.RemoteURL = url
	}
	if key := os.Getenv("LORA_TRAIN_API_KEY"); key != "" {
		cfg.RemoteKey = key
	}
	if script := os.Getenv("LORA_TRAIN_SCRIPT"); script != "" {
		cfg.ScriptPath = script
	}
	if t := os.Getenv("LORA_TRAIN_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv("LORA_TRAIN_SEED"); v != "" {
		var seed int
		if _, err := fmt.Sscanf(v, "%d", &seed); err == nil && seed > 0 {
			cfg.Seed = seed
		}
	}
	if v := os.Getenv("LORA_TRAIN_MAX_SEQ_LENGTH"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 64 {
			cfg.MaxSeqLength = n
		}
	}
	if os.Getenv("LORA_TRAIN_TRUST_REMOTE_CODE") == "true" {
		cfg.TrustRemoteCode = true
	}
	if v := os.Getenv("LORA_TRAIN_TARGET_MODULES"); v != "" {
		var modules []string
		for _, m := range strings.Split(v, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				modules = append(modules, m)
			}
		}
		if len(modules) > 0 {
			cfg.TargetModules = modules
		}
	}
	return cfg
}

// LoRATrainer implements the TrainFunc signature using either a remote
// training API or a local Python subprocess. In "auto" mode it prefers
// remote when configured, falling back to local.
type LoRATrainer struct {
	cfg    TrainerConfig
	client *http.Client
}

func NewLoRATrainer(cfg TrainerConfig) *LoRATrainer {
	return &LoRATrainer{
		cfg: cfg,
		client: &http.Client{
			Timeout: 0,
		},
	}
}

// TrainFunc returns a TrainFunc suitable for LoRAScheduler.SetTrainFunc().
func (lt *LoRATrainer) TrainFunc() TrainFunc {
	return lt.train
}

func (lt *LoRATrainer) train(ctx context.Context, job TrainJob) (*TrainResult, error) {
	if err := validateTrainJob(job); err != nil {
		return nil, fmt.Errorf("lora_trainer: invalid job: %w", err)
	}

	lt.applyDefaults(&job)

	mode := lt.resolveMode()
	slog.Info("lora_trainer: starting",
		"mode", mode,
		"adapter", job.AdapterName,
		"base_model", job.BaseModel,
		"data_path", job.DataPath,
		"epochs", job.NumEpochs,
		"rank", job.LoRARank,
		"seed", job.Seed,
	)

	switch mode {
	case TrainerModeRemote:
		return lt.trainRemote(ctx, job)
	case TrainerModeLocal:
		return lt.trainLocal(ctx, job)
	default:
		return nil, fmt.Errorf("lora_trainer: unknown mode %q", mode)
	}
}

func validateTrainJob(job TrainJob) error {
	var errs []string
	if job.BaseModel == "" {
		errs = append(errs, "base_model is required")
	}
	if job.DataPath == "" {
		errs = append(errs, "data_path is required")
	}
	if job.OutputDir == "" {
		errs = append(errs, "output_dir is required")
	}
	if job.AdapterName == "" {
		errs = append(errs, "adapter_name is required")
	}
	if strings.Contains(job.AdapterName, "..") {
		errs = append(errs, "adapter_name contains path traversal")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (lt *LoRATrainer) applyDefaults(job *TrainJob) {
	if job.NumEpochs <= 0 {
		job.NumEpochs = 3
	}
	if job.LoRARank <= 0 {
		job.LoRARank = 16
	}
	if job.LearningRate <= 0 {
		job.LearningRate = 2e-4
	}
	if job.MaxSeqLength <= 0 {
		job.MaxSeqLength = lt.cfg.MaxSeqLength
	}
	if job.Seed == 0 {
		job.Seed = lt.cfg.Seed
	}
	if len(job.TargetModules) == 0 && len(lt.cfg.TargetModules) > 0 {
		job.TargetModules = lt.cfg.TargetModules
	}
	if !job.TrustRemoteCode && lt.cfg.TrustRemoteCode {
		job.TrustRemoteCode = true
	}
}

func (lt *LoRATrainer) resolveMode() TrainerMode {
	if lt.cfg.Mode != TrainerModeAuto {
		return lt.cfg.Mode
	}
	if lt.cfg.RemoteURL != "" {
		return TrainerModeRemote
	}
	return TrainerModeLocal
}

// ── Remote Training ──

type remoteTrainRequest struct {
	BaseModel   string            `json:"base_model"`
	DataPath    string            `json:"data_path"`
	AdapterName string            `json:"adapter_name"`
	OutputDir   string            `json:"output_dir"`
	Config      remoteTrainParams `json:"config"`
}

type remoteTrainParams struct {
	LoRARank        int      `json:"lora_rank"`
	NumEpochs       int      `json:"num_epochs"`
	LearningRate    float64  `json:"learning_rate"`
	MaxSeqLength    int      `json:"max_seq_length"`
	Seed            int      `json:"seed"`
	TargetModules   []string `json:"target_modules,omitempty"`
	TrustRemoteCode bool     `json:"trust_remote_code"`
	ResumeFrom      string   `json:"resume_from,omitempty"`
}

type remoteTrainResponse struct {
	JobID string `json:"job_id"`
	Error string `json:"error,omitempty"`
}

type remoteJobStatus struct {
	Status      string  `json:"status"`
	AdapterPath string  `json:"adapter_path,omitempty"`
	FinalLoss   float64 `json:"final_loss,omitempty"`
	Samples     int     `json:"samples,omitempty"`
	Epochs      int     `json:"epochs,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	Error       string  `json:"error,omitempty"`
}

func (lt *LoRATrainer) trainRemote(ctx context.Context, job TrainJob) (*TrainResult, error) {
	ctx, cancel := context.WithTimeout(ctx, lt.cfg.Timeout)
	defer cancel()

	jobID, err := lt.submitRemoteJob(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("lora_trainer: submit remote job: %w", err)
	}
	slog.Info("lora_trainer: remote job submitted", "job_id", jobID)

	return lt.pollRemoteJob(ctx, jobID, job)
}

func (lt *LoRATrainer) submitRemoteJob(ctx context.Context, job TrainJob) (string, error) {
	reqBody := remoteTrainRequest{
		BaseModel:   job.BaseModel,
		DataPath:    job.DataPath,
		AdapterName: job.AdapterName,
		OutputDir:   job.OutputDir,
		Config: remoteTrainParams{
			LoRARank:        job.LoRARank,
			NumEpochs:       job.NumEpochs,
			LearningRate:    job.LearningRate,
			MaxSeqLength:    job.MaxSeqLength,
			Seed:            job.Seed,
			TargetModules:   job.TargetModules,
			TrustRemoteCode: job.TrustRemoteCode,
			ResumeFrom:      job.ResumeFrom,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", lt.cfg.RemoteURL+"/v1/train", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	lt.setHeaders(req)

	resp, err := lt.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxStderrCapture))
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, string(errBody))
	}

	var result remoteTrainResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("server error: %s", result.Error)
	}
	if result.JobID == "" {
		return "", fmt.Errorf("server returned empty job_id")
	}

	return result.JobID, nil
}

func (lt *LoRATrainer) pollRemoteJob(ctx context.Context, jobID string, job TrainJob) (*TrainResult, error) {
	ticker := time.NewTicker(lt.cfg.PollInterval)
	defer ticker.Stop()

	consecutiveErrors := 0
	const maxPollErrors = 10

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("lora_trainer: timeout waiting for job %s", jobID)
		case <-ticker.C:
			status, err := lt.getRemoteJobStatus(ctx, jobID)
			if err != nil {
				consecutiveErrors++
				slog.Warn("lora_trainer: poll error",
					"job_id", jobID,
					"err", err,
					"consecutive", consecutiveErrors,
				)
				if consecutiveErrors >= maxPollErrors {
					return nil, fmt.Errorf("lora_trainer: %d consecutive poll failures for job %s: %w",
						maxPollErrors, jobID, err)
				}
				continue
			}
			consecutiveErrors = 0

			switch status.Status {
			case "completed":
				adapterPath := status.AdapterPath
				if adapterPath == "" {
					adapterPath = job.OutputDir
				}
				return &TrainResult{
					AdapterName: job.AdapterName,
					AdapterPath: adapterPath,
					Samples:     status.Samples,
					Epochs:      status.Epochs,
					FinalLoss:   status.FinalLoss,
					Duration:    time.Duration(status.DurationSec * float64(time.Second)),
					Success:     true,
				}, nil

			case "failed":
				return &TrainResult{
					AdapterName: job.AdapterName,
					Success:     false,
					Error:       status.Error,
				}, nil

			case "queued", "running":
				slog.Debug("lora_trainer: job in progress", "job_id", jobID, "status", status.Status)

			default:
				slog.Warn("lora_trainer: unknown job status", "job_id", jobID, "status", status.Status)
			}
		}
	}
}

func (lt *LoRATrainer) getRemoteJobStatus(ctx context.Context, jobID string) (*remoteJobStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", lt.cfg.RemoteURL+"/v1/train/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	lt.setHeaders(req)

	resp, err := lt.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxStderrCapture))
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(errBody))
	}

	var status remoteJobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode status: %w", err)
	}
	return &status, nil
}

func (lt *LoRATrainer) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if lt.cfg.RemoteKey != "" {
		req.Header.Set("Authorization", "Bearer "+lt.cfg.RemoteKey)
	}
}

// ── Local Training ──

type localTrainArgs struct {
	BaseModel       string   `json:"base_model"`
	DataPath        string   `json:"data_path"`
	OutputDir       string   `json:"output_dir"`
	AdapterName     string   `json:"adapter_name"`
	NumEpochs       int      `json:"num_epochs"`
	LoRARank        int      `json:"lora_rank"`
	LearningRate    float64  `json:"learning_rate"`
	MaxSeqLength    int      `json:"max_seq_length"`
	Seed            int      `json:"seed"`
	TargetModules   []string `json:"target_modules,omitempty"`
	TrustRemoteCode bool     `json:"trust_remote_code"`
	ResumeFrom      string   `json:"resume_from,omitempty"`
}

type localTrainOutput struct {
	AdapterPath     string            `json:"adapter_path"`
	FinalLoss       float64           `json:"final_loss"`
	Samples         int               `json:"samples"`
	Epochs          int               `json:"epochs"`
	Success         bool              `json:"success"`
	Error           string            `json:"error,omitempty"`
	TrainableParams int               `json:"trainable_params,omitempty"`
	TotalParams     int               `json:"total_params,omitempty"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	DataStats       map[string]int    `json:"data_stats,omitempty"`
}

func (lt *LoRATrainer) trainLocal(ctx context.Context, job TrainJob) (*TrainResult, error) {
	script, err := filepath.Abs(lt.cfg.ScriptPath)
	if err != nil {
		return nil, fmt.Errorf("lora_trainer: resolve script path: %w", err)
	}
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("lora_trainer: training script not found: %s", script)
	}

	if _, err := os.Stat(job.DataPath); err != nil {
		return nil, fmt.Errorf("lora_trainer: training data not found: %s", job.DataPath)
	}

	ctx, cancel := context.WithTimeout(ctx, lt.cfg.Timeout)
	defer cancel()

	args := localTrainArgs{
		BaseModel:       job.BaseModel,
		DataPath:        job.DataPath,
		OutputDir:       job.OutputDir,
		AdapterName:     job.AdapterName,
		NumEpochs:       job.NumEpochs,
		LoRARank:        job.LoRARank,
		LearningRate:    job.LearningRate,
		MaxSeqLength:    job.MaxSeqLength,
		Seed:            job.Seed,
		TargetModules:   job.TargetModules,
		TrustRemoteCode: job.TrustRemoteCode,
		ResumeFrom:      job.ResumeFrom,
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("lora_trainer: marshal args: %w", err)
	}

	if err := os.MkdirAll(job.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("lora_trainer: create output dir: %w", err)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "python3", script, "--json-args", string(argsJSON))

	var stdout bytes.Buffer
	stderr := &limitedWriter{max: maxStderrCapture}
	cmd.Stdout = &stdout
	cmd.Stderr = stderr

	slog.Info("lora_trainer: running local script",
		"script", script,
		"adapter", job.AdapterName,
		"data_path", job.DataPath,
	)

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		slog.Error("lora_trainer: local training failed",
			"err", err,
			"stderr", stderrStr,
			"duration", time.Since(start),
		)
		return &TrainResult{
			AdapterName: job.AdapterName,
			Duration:    time.Since(start),
			Success:     false,
			Error:       fmt.Sprintf("script failed: %v; stderr: %s", err, stderrStr),
		}, nil
	}

	duration := time.Since(start)

	var output localTrainOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		slog.Warn("lora_trainer: failed to parse script output, checking output dir",
			"raw_len", stdout.Len(),
			"err", err,
		)
		adapterPath := filepath.Join(job.OutputDir, job.AdapterName)
		if _, serr := os.Stat(adapterPath); serr != nil {
			adapterPath = job.OutputDir
		}
		return &TrainResult{
			AdapterName: job.AdapterName,
			AdapterPath: adapterPath,
			Duration:    duration,
			Success:     true,
		}, nil
	}

	if !output.Success {
		return &TrainResult{
			AdapterName: job.AdapterName,
			Duration:    duration,
			Success:     false,
			Error:       output.Error,
		}, nil
	}

	adapterPath := output.AdapterPath
	if adapterPath == "" {
		adapterPath = job.OutputDir
	}

	slog.Info("lora_trainer: local training succeeded",
		"adapter", job.AdapterName,
		"loss", output.FinalLoss,
		"samples", output.Samples,
		"duration", duration,
		"trainable_params", output.TrainableParams,
	)

	return &TrainResult{
		AdapterName: job.AdapterName,
		AdapterPath: adapterPath,
		Samples:     output.Samples,
		Epochs:      output.Epochs,
		FinalLoss:   output.FinalLoss,
		Duration:    duration,
		Success:     true,
	}, nil
}

// limitedWriter captures up to max bytes, discarding the rest.
type limitedWriter struct {
	buf bytes.Buffer
	max int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	remaining := lw.max - lw.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	lw.buf.Write(p)
	return len(p), nil
}

func (lw *limitedWriter) String() string {
	return lw.buf.String()
}
