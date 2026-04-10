package localbrain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AdaptiveLoop 管理用户偏好的在线学习和离线 LoRA 微调数据导出。
//
// 工作流：
//   1. 每次请求后 RecordFeedback() 记录 (query, intent, tier, satisfied)
//   2. 在线部分：立即更新 userPatterns，下次分类时影响路由决策
//   3. 离线部分：定期 ExportForFinetune() 导出 JSONL，用户自行 LoRA 微调
//   4. 微调后的模型替换 LocalBrain.client，形成正向循环
type AdaptiveLoop struct {
	brain    *LocalBrain
	dataDir  string
	mu       sync.RWMutex
	sessions map[string]*SessionAdaptation // tenantID → in-session adaptation
}

// SessionAdaptation 会话级在线适应。
type SessionAdaptation struct {
	TenantID      string
	CorrectRoutes int // 路由正确次数
	WrongRoutes   int // 路由错误次数（用户手动纠正）
	StartedAt     time.Time

	// 短期校准：如果同一会话中小模型多次失败，临时提高升级阈值
	tempBoost float64
}

// NewAdaptiveLoop 创建适应性学习循环。
func NewAdaptiveLoop(brain *LocalBrain, dataDir string) *AdaptiveLoop {
	return &AdaptiveLoop{
		brain:    brain,
		dataDir:  dataDir,
		sessions: make(map[string]*SessionAdaptation),
	}
}

// OnSessionStart 会话开始时初始化适应状态。
func (al *AdaptiveLoop) OnSessionStart(tenantID string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.sessions[tenantID] = &SessionAdaptation{
		TenantID:  tenantID,
		StartedAt: time.Now(),
	}
}

// OnSessionEnd 会话结束时清理，但保留聚合数据。
func (al *AdaptiveLoop) OnSessionEnd(tenantID string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.sessions, tenantID)
}

// GetTempBoost 获取当前会话的临时置信度提升（用于阈值调整）。
func (al *AdaptiveLoop) GetTempBoost(tenantID string) float64 {
	al.mu.RLock()
	defer al.mu.RUnlock()
	if s, ok := al.sessions[tenantID]; ok {
		return s.tempBoost
	}
	return 0
}

// OnRouteCorrection 用户手动纠正路由时调用。
// 例如：小模型说"简单"，但用户要求用"expert"。
func (al *AdaptiveLoop) OnRouteCorrection(tenantID string, wasLocal bool) {
	al.mu.Lock()
	defer al.mu.Unlock()
	s, ok := al.sessions[tenantID]
	if !ok {
		return
	}
	s.WrongRoutes++
	// 连续错误 → 临时提高升级阈值
	if s.WrongRoutes > 2 {
		s.tempBoost += 0.1
		if s.tempBoost > 0.3 {
			s.tempBoost = 0.3
		}
	}
}

// OnRouteSuccess 路由成功时调用。
func (al *AdaptiveLoop) OnRouteSuccess(tenantID string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	s, ok := al.sessions[tenantID]
	if !ok {
		return
	}
	s.CorrectRoutes++
	// 连续成功 → 缓慢降低 tempBoost
	if s.CorrectRoutes > 3 && s.tempBoost > 0 {
		s.tempBoost -= 0.05
		if s.tempBoost < 0 {
			s.tempBoost = 0
		}
	}
}

// ExportForFinetune 导出 LoRA 微调用 JSONL 文件。
// 格式兼容 Hugging Face / Unsloth / LLaMA-Factory 的标准训练格式。
func (al *AdaptiveLoop) ExportForFinetune(tenantID string) (string, error) {
	samples := al.brain.ExportTrainingData(tenantID)
	if len(samples) == 0 {
		return "", fmt.Errorf("no training data for tenant %s", tenantID)
	}

	if err := os.MkdirAll(al.dataDir, 0750); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	filename := fmt.Sprintf("lora_data_%s_%s.jsonl", tenantID, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(al.dataDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, sample := range samples {
		// 转换为 instruction-following 格式
		record := map[string]string{
			"instruction": "Classify the following user query into intent category, complexity level, and whether tools are needed.",
			"input":       sample.Input,
			"output": fmt.Sprintf(`{"category":"%s","complexity":"%s","confidence":%.1f,"need_tools":%v}`,
				sample.Intent.Category,
				sample.Intent.Complexity,
				sample.Intent.Confidence,
				sample.Intent.NeedTools),
		}
		if err := enc.Encode(record); err != nil {
			return "", fmt.Errorf("encode sample: %w", err)
		}
	}

	return outPath, nil
}
