package localbrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultTrainerConfig(t *testing.T) {
	cfg := DefaultTrainerConfig()
	if cfg.Mode != TrainerModeAuto {
		t.Errorf("Mode = %s, want auto", cfg.Mode)
	}
	if cfg.Timeout != 2*time.Hour {
		t.Errorf("Timeout = %v, want 2h", cfg.Timeout)
	}
	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
	if cfg.ScriptPath != "scripts/lora_train.py" {
		t.Errorf("ScriptPath = %s", cfg.ScriptPath)
	}
}

func TestTrainerConfigFromEnv(t *testing.T) {
	t.Setenv("LORA_TRAIN_MODE", "remote")
	t.Setenv("LORA_TRAIN_API_URL", "https://train.example.com")
	t.Setenv("LORA_TRAIN_API_KEY", "sk-test")
	t.Setenv("LORA_TRAIN_SCRIPT", "/opt/train.py")
	t.Setenv("LORA_TRAIN_TIMEOUT", "3h")

	cfg := TrainerConfigFromEnv()
	if cfg.Mode != TrainerModeRemote {
		t.Errorf("Mode = %s, want remote", cfg.Mode)
	}
	if cfg.RemoteURL != "https://train.example.com" {
		t.Errorf("RemoteURL = %s", cfg.RemoteURL)
	}
	if cfg.RemoteKey != "sk-test" {
		t.Errorf("RemoteKey = %s", cfg.RemoteKey)
	}
	if cfg.ScriptPath != "/opt/train.py" {
		t.Errorf("ScriptPath = %s", cfg.ScriptPath)
	}
	if cfg.Timeout != 3*time.Hour {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
}

func TestResolveMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     TrainerMode
		url      string
		expected TrainerMode
	}{
		{"explicit remote", TrainerModeRemote, "", TrainerModeRemote},
		{"explicit local", TrainerModeLocal, "", TrainerModeLocal},
		{"auto with url", TrainerModeAuto, "http://x", TrainerModeRemote},
		{"auto without url", TrainerModeAuto, "", TrainerModeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lt := NewLoRATrainer(TrainerConfig{Mode: tt.mode, RemoteURL: tt.url})
			got := lt.resolveMode()
			if got != tt.expected {
				t.Errorf("resolveMode() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestRemoteTrain_FullFlow(t *testing.T) {
	phase := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/train":
			var req remoteTrainRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.BaseModel == "" {
				t.Error("empty base_model in request")
			}
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(remoteTrainResponse{JobID: "job-123"})

		case r.Method == "GET" && r.URL.Path == "/v1/train/job-123":
			phase++
			var status remoteJobStatus
			if phase < 2 {
				status = remoteJobStatus{Status: "running"}
			} else {
				status = remoteJobStatus{
					Status:      "completed",
					AdapterPath: "/output/adapter-v1",
					FinalLoss:   0.15,
					Samples:     100,
					Epochs:      3,
					DurationSec: 120,
				}
			}
			json.NewEncoder(w).Encode(status)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	trainer := NewLoRATrainer(TrainerConfig{
		Mode:         TrainerModeRemote,
		RemoteURL:    srv.URL,
		RemoteKey:    "test-key",
		Timeout:      10 * time.Second,
		PollInterval: 50 * time.Millisecond,
	})

	job := TrainJob{
		BaseModel:    "qwen-3.5-4b",
		DataPath:     "/data/train.jsonl",
		OutputDir:    "/output",
		AdapterName:  "adapter-v1",
		NumEpochs:    3,
		LoRARank:     16,
		LearningRate: 2e-4,
	}

	result, err := trainer.train(context.Background(), job)
	if err != nil {
		t.Fatalf("train failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("training unsuccessful: %s", result.Error)
	}
	if result.AdapterPath != "/output/adapter-v1" {
		t.Errorf("AdapterPath = %s", result.AdapterPath)
	}
	if result.FinalLoss != 0.15 {
		t.Errorf("FinalLoss = %f", result.FinalLoss)
	}
	if result.Samples != 100 {
		t.Errorf("Samples = %d", result.Samples)
	}
}

func TestRemoteTrain_JobFailed(t *testing.T) {
	tmpData := filepath.Join(t.TempDir(), "train.jsonl")
	os.WriteFile(tmpData, []byte("{}\n"), 0644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/train":
			json.NewEncoder(w).Encode(remoteTrainResponse{JobID: "job-fail"})
		case r.Method == "GET" && r.URL.Path == "/v1/train/job-fail":
			json.NewEncoder(w).Encode(remoteJobStatus{
				Status: "failed",
				Error:  "GPU OOM",
			})
		}
	}))
	defer srv.Close()

	trainer := NewLoRATrainer(TrainerConfig{
		Mode:         TrainerModeRemote,
		RemoteURL:    srv.URL,
		Timeout:      5 * time.Second,
		PollInterval: 50 * time.Millisecond,
	})

	result, err := trainer.train(context.Background(), TrainJob{
		BaseModel:   "qwen-3.5-4b",
		DataPath:    tmpData,
		OutputDir:   t.TempDir(),
		AdapterName: "fail-adapter",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if result.Error != "GPU OOM" {
		t.Errorf("Error = %s, want GPU OOM", result.Error)
	}
}

func TestRemoteTrain_SubmitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service down"))
	}))
	defer srv.Close()

	trainer := NewLoRATrainer(TrainerConfig{
		Mode:      TrainerModeRemote,
		RemoteURL: srv.URL,
		Timeout:   5 * time.Second,
	})

	tmpData := filepath.Join(t.TempDir(), "train.jsonl")
	os.WriteFile(tmpData, []byte("{}\n"), 0644)

	_, err := trainer.train(context.Background(), TrainJob{
		BaseModel:   "qwen-3.5-4b",
		DataPath:    tmpData,
		OutputDir:   t.TempDir(),
		AdapterName: "test",
	})
	if err == nil {
		t.Fatal("expected error on 503")
	}
}

func TestRemoteTrain_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(remoteTrainResponse{JobID: "j1"})
		} else {
			json.NewEncoder(w).Encode(remoteJobStatus{Status: "completed", AdapterPath: "/out"})
		}
	}))
	defer srv.Close()

	tmpData := filepath.Join(t.TempDir(), "train.jsonl")
	os.WriteFile(tmpData, []byte("{}\n"), 0644)

	trainer := NewLoRATrainer(TrainerConfig{
		Mode:         TrainerModeRemote,
		RemoteURL:    srv.URL,
		RemoteKey:    "sk-secret",
		Timeout:      5 * time.Second,
		PollInterval: 50 * time.Millisecond,
	})

	trainer.train(context.Background(), TrainJob{
		BaseModel:   "m",
		DataPath:    tmpData,
		OutputDir:   t.TempDir(),
		AdapterName: "a",
	})

	if gotAuth != "Bearer sk-secret" {
		t.Errorf("Authorization = %s, want Bearer sk-secret", gotAuth)
	}
}

func TestLocalTrain_ScriptNotFound(t *testing.T) {
	trainer := NewLoRATrainer(TrainerConfig{
		Mode:       TrainerModeLocal,
		ScriptPath: "/nonexistent/train.py",
		Timeout:    5 * time.Second,
	})

	_, err := trainer.train(context.Background(), TrainJob{BaseModel: "m"})
	if err == nil {
		t.Fatal("expected error for missing script")
	}
}

func TestLocalTrain_ScriptSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	dataFile := filepath.Join(tmpDir, "train.jsonl")
	os.WriteFile(dataFile, []byte("{\"instruction\":\"test\",\"output\":\"ok\"}\n"), 0644)

	scriptContent := `
import sys, json
args = json.loads(sys.argv[2])
result = {
    "adapter_path": args["output_dir"] + "/" + args["adapter_name"],
    "final_loss": 0.12,
    "samples": 50,
    "epochs": 3,
    "success": True
}
print(json.dumps(result))
`
	scriptPath := filepath.Join(tmpDir, "train.py")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	outputDir := filepath.Join(tmpDir, "output")
	trainer := NewLoRATrainer(TrainerConfig{
		Mode:       TrainerModeLocal,
		ScriptPath: scriptPath,
		Timeout:    30 * time.Second,
	})

	result, err := trainer.train(context.Background(), TrainJob{
		BaseModel:   "qwen-3.5-4b",
		DataPath:    dataFile,
		OutputDir:   outputDir,
		AdapterName: "test-adapter",
		NumEpochs:   3,
		LoRARank:    16,
	})
	if err != nil {
		t.Fatalf("train failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("training unsuccessful: %s", result.Error)
	}
	if result.FinalLoss != 0.12 {
		t.Errorf("FinalLoss = %f, want 0.12", result.FinalLoss)
	}
	if result.Samples != 50 {
		t.Errorf("Samples = %d, want 50", result.Samples)
	}
}

func TestLocalTrain_ScriptFailure(t *testing.T) {
	tmpDir := t.TempDir()

	dataFile := filepath.Join(tmpDir, "train.jsonl")
	os.WriteFile(dataFile, []byte("{}\n"), 0644)

	scriptContent := `
import sys
print("some error", file=sys.stderr)
sys.exit(1)
`
	scriptPath := filepath.Join(tmpDir, "bad_train.py")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	trainer := NewLoRATrainer(TrainerConfig{
		Mode:       TrainerModeLocal,
		ScriptPath: scriptPath,
		Timeout:    10 * time.Second,
	})

	result, err := trainer.train(context.Background(), TrainJob{
		BaseModel:   "qwen-3.5-4b",
		DataPath:    dataFile,
		OutputDir:   tmpDir,
		AdapterName: "fail",
	})
	if err != nil {
		t.Fatalf("unexpected error (should return result): %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestValidateTrainJob(t *testing.T) {
	tests := []struct {
		name    string
		job     TrainJob
		wantErr bool
	}{
		{"valid", TrainJob{BaseModel: "m", DataPath: "/d", OutputDir: "/o", AdapterName: "a"}, false},
		{"missing base_model", TrainJob{DataPath: "/d", OutputDir: "/o", AdapterName: "a"}, true},
		{"missing data_path", TrainJob{BaseModel: "m", OutputDir: "/o", AdapterName: "a"}, true},
		{"path traversal", TrainJob{BaseModel: "m", DataPath: "/d", OutputDir: "/o", AdapterName: "../evil"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTrainJob(tt.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTrainJob() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLimitedWriter(t *testing.T) {
	lw := &limitedWriter{max: 10}
	lw.Write([]byte("hello"))
	lw.Write([]byte(" world this is long"))
	if lw.buf.Len() > 10 {
		t.Errorf("limitedWriter exceeded max: len=%d", lw.buf.Len())
	}
	if !strings.HasPrefix(lw.String(), "hello") {
		t.Errorf("unexpected content: %s", lw.String())
	}
}

func TestTrainFuncMethod(t *testing.T) {
	trainer := NewLoRATrainer(DefaultTrainerConfig())
	fn := trainer.TrainFunc()
	if fn == nil {
		t.Fatal("TrainFunc() returned nil")
	}
}
