package skillanomaly

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSkillAnomalyHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 Skill Anomaly routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if route.Path == "" || route.Handler == nil || len(methods) == 0 {
			t.Fatalf("route must declare path, handler and method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}
	expected := map[string][]string{
		"/v1/skill-anomaly/status":                     {http.MethodGet},
		"/v1/skill-anomaly/events":                     {http.MethodGet, http.MethodPost},
		"/v1/skill-anomaly/profiles":                   {http.MethodGet},
		"/v1/skill-anomaly/profiles/":                  {http.MethodGet},
		"/v1/skill-anomaly/detect":                     {http.MethodPost},
		"/v1/skill-anomaly/audit-hook/plan":            {http.MethodPost},
		"/v1/skill-anomaly/approval-queue/writeback":   {http.MethodPost},
		"/v1/skill-anomaly/approval-queue/bridge/plan": {http.MethodPost},
		"/v1/skill-anomaly/evidence/":                  {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestSkillAnomalyObserveDetectAndEvidence(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	counter := 0
	h := New(Config{DataDir: t.TempDir(), Policy: DetectionPolicy{MinObservations: 3, WindowSize: 10}, Now: func() time.Time {
		counter++
		return now.Add(time.Duration(counter) * time.Minute)
	}})

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"text_processing","action":"read_file","params":{"path":"notes.md"},"success":true,"duration_ms":100}`))
		h.Events(w, req)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "observed") {
			t.Fatalf("observe baseline status=%d body=%s", w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/detect", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"dry_run":true}`))
	h.Detect(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "needs_approval") || !strings.Contains(w.Body.String(), "new_action") || !strings.Contains(w.Body.String(), "new_param_keys") {
		t.Fatalf("detect status=%d body=%s", w.Code, w.Body.String())
	}
	var detected struct {
		Result DetectionResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&detected); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if !detected.Result.NeedsApproval || detected.Result.Score < 7 {
		t.Fatalf("expected high anomaly score and needs approval, got %#v", detected.Result)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/audit-hook/plan", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review anomalous shell execution"}`))
	h.AuditHookPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_plan") || !strings.Contains(w.Body.String(), "trust_mutation") || !strings.Contains(w.Body.String(), "merkle_append_ready") {
		t.Fatalf("audit hook plan status=%d body=%s", w.Code, w.Body.String())
	}
	var auditPlan struct {
		Plan AuditHookPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&auditPlan); err != nil {
		t.Fatalf("decode audit hook plan: %v", err)
	}
	if !auditPlan.Plan.ApprovalRequired || auditPlan.Plan.AuditHookReady || auditPlan.Plan.TrustMutationReady || auditPlan.Plan.ApprovalWritebackReady {
		t.Fatalf("expected non-destructive approval plan boundaries, got %#v", auditPlan.Plan)
	}
	if auditPlan.Plan.TrustMutation.Delta >= 0 || auditPlan.Plan.ApprovalQueue.Reason != "review anomalous shell execution" {
		t.Fatalf("unexpected trust/approval plan: %#v", auditPlan.Plan)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/writeback", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review anomalous shell execution","request_id":"skill-anomaly-custom","request_key":"skill-anomaly-custom-key"}`))
	h.ApprovalQueueWriteback(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "approval_queue_store") || !strings.Contains(w.Body.String(), "approval-queue-store.json") {
		t.Fatalf("approval queue writeback status=%d body=%s", w.Code, w.Body.String())
	}
	var writebackResp struct {
		Writeback ApprovalQueueWritebackReport `json:"writeback"`
	}
	if err := json.NewDecoder(w.Body).Decode(&writebackResp); err != nil {
		t.Fatalf("decode approval queue writeback: %v", err)
	}
	writeback := writebackResp.Writeback
	if !writeback.ApprovalWritebackReady || !writeback.WritesApprovalQueue || !writeback.WritesApprovalQueueFile || writeback.AuditHookReady || writeback.TrustMutationReady || writeback.MerkleAppendReady || !writeback.ExecutionBlocked || writeback.ActionAllowed {
		t.Fatalf("approval queue writeback should persist only the pack-local queue and keep audit/trust execution blocked: %#v", writeback)
	}
	if writeback.ApprovalQueueRecord.RequestKey != "skill-anomaly-custom-key" || writeback.ApprovalQueueStore.RecordCount != 1 {
		t.Fatalf("approval queue writeback should expose persisted store and record: %#v", writeback)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/bridge/plan", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review anomalous shell execution","request_id":"skill-anomaly-custom","request_key":"skill-anomaly-custom-key"}`))
	h.ApprovalQueueBridgePlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval-manager-bridge-plan.json") || !strings.Contains(w.Body.String(), "global_approval_enqueue_ready") {
		t.Fatalf("approval manager bridge plan status=%d body=%s", w.Code, w.Body.String())
	}
	var bridgeResp struct {
		Plan ApprovalManagerBridgePlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&bridgeResp); err != nil {
		t.Fatalf("decode approval manager bridge plan: %v", err)
	}
	bridgePlan := bridgeResp.Plan
	if !bridgePlan.ApprovalManagerBridgePlanReady || bridgePlan.GlobalApprovalEnqueueReady || bridgePlan.MerkleAppendReady || bridgePlan.TrustMutationReady || bridgePlan.ActionReleaseReady {
		t.Fatalf("bridge plan should be ready but keep global side effects blocked: %#v", bridgePlan)
	}
	if !bridgePlan.SourceQueueRecordPersisted || bridgePlan.SourceApprovalQueueRecord.RequestKey != "skill-anomaly-custom-key" || bridgePlan.ProposedGlobalApprovalRequest.Category != "code_execution" || bridgePlan.ProposedGlobalApprovalRequest.RiskLevel != "critical" {
		t.Fatalf("bridge plan should map persisted queue record into global approval request shape: %#v", bridgePlan)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/profiles/text_processing", nil)
	h.ProfileDetail(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "read_file") {
		t.Fatalf("profile status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/evidence/text_processing", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-skill-anomaly-evidence") || !strings.Contains(w.Body.String(), "detection-policy.json") || !strings.Contains(w.Body.String(), "audit-hook-plan.json") || !strings.Contains(w.Body.String(), "trust-mutation-plan.json") || !strings.Contains(w.Body.String(), "approval-queue-store.json") || !strings.Contains(w.Body.String(), "approval-queue-record.json") || !strings.Contains(w.Body.String(), "approval-manager-bridge-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSkillAnomalyApprovalQueueWritebackPersistsPackLocalStore(t *testing.T) {
	now := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	counter := 0
	h := New(Config{DataDir: t.TempDir(), Policy: DetectionPolicy{MinObservations: 1, WindowSize: 10}, Now: func() time.Time {
		counter++
		return now.Add(time.Duration(counter) * time.Minute)
	}})
	baseline := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"writer","action":"read_file","params":{"path":"notes.md"},"success":true,"duration_ms":100}`))
	w := httptest.NewRecorder()
	h.Events(w, baseline)
	if w.Code != http.StatusCreated {
		t.Fatalf("baseline status=%d body=%s", w.Code, w.Body.String())
	}
	body := `{"skill_slug":"writer","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review suspicious writer action","request_id":"skill-anomaly-writer","request_key":"skill-anomaly-writer-key"}`
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/writeback", strings.NewReader(body))
	h.ApprovalQueueWriteback(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("approval queue writeback status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/writeback", strings.NewReader(strings.Replace(body, "review suspicious writer action", "review suspicious writer action again", 1)))
	h.ApprovalQueueWriteback(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("idempotent approval queue writeback status=%d body=%s", w.Code, w.Body.String())
	}
	var stored struct {
		PackID      string                `json:"pack_id"`
		QueueName   string                `json:"queue_name"`
		Format      string                `json:"format"`
		RecordCount int                   `json:"record_count"`
		Records     []ApprovalQueueRecord `json:"records"`
	}
	data, err := os.ReadFile(h.approvalQueueStorePath())
	if err != nil {
		t.Fatalf("read approval queue store: %v", err)
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode approval queue store: %v", err)
	}
	if stored.PackID != PackID || stored.QueueName != "skill_anomaly_approval" || stored.Format != "json-skill-anomaly-approval-queue-store" || stored.RecordCount != 1 || len(stored.Records) != 1 {
		t.Fatalf("approval queue store format mismatch: %#v", stored)
	}
	if stored.Records[0].RequestKey != "skill-anomaly-writer-key" || !stored.Records[0].WritesApprovalQueueFile || !stored.Records[0].ExecutionBlocked || stored.Records[0].TrustMutationReady || stored.Records[0].MerkleAppendReady {
		t.Fatalf("stored queue record should keep audit/trust execution blocked: %#v", stored.Records[0])
	}
	if stored.Records[0].Reason != "review suspicious writer action again" {
		t.Fatalf("same request key should upsert queue record: %#v", stored.Records[0])
	}
}

func TestSkillAnomalyApprovalManagerBridgePlanIsPlanOnly(t *testing.T) {
	now := time.Date(2026, 5, 15, 20, 0, 0, 0, time.UTC)
	counter := 0
	h := New(Config{DataDir: t.TempDir(), Policy: DetectionPolicy{MinObservations: 1, WindowSize: 10}, Now: func() time.Time {
		counter++
		return now.Add(time.Duration(counter) * time.Minute)
	}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"writer","action":"read_file","params":{"path":"notes.md"},"success":true,"duration_ms":100}`))
	h.Events(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("baseline status=%d body=%s", w.Code, w.Body.String())
	}

	body := `{"skill_slug":"writer","action":"delete_file","params":{"path":"prod.db","force":true},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review suspicious writer mutation","request_id":"skill-anomaly-writer-bridge","request_key":"skill-anomaly-writer-bridge-key"}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/bridge/plan", strings.NewReader(body))
	h.ApprovalQueueBridgePlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bridge plan status=%d body=%s", w.Code, w.Body.String())
	}
	var preview struct {
		Plan ApprovalManagerBridgePlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&preview); err != nil {
		t.Fatalf("decode bridge preview: %v", err)
	}
	if preview.Plan.SourceQueueRecordPersisted || preview.Plan.GlobalApprovalEnqueueReady || preview.Plan.ProposedGlobalApprovalRequest.GlobalApprovalEnqueueReady || preview.Plan.ProposedGlobalApprovalRequest.ActionReleaseReady {
		t.Fatalf("bridge preview should not mutate global approval state: %#v", preview.Plan)
	}
	if preview.Plan.ProposedGlobalApprovalRequest.Category != "data_mutation" || preview.Plan.ProposedGlobalApprovalRequest.QueueName != "global_approval_manager" {
		t.Fatalf("bridge preview should classify data mutations for global approval: %#v", preview.Plan.ProposedGlobalApprovalRequest)
	}
	if _, err := os.Stat(h.approvalQueueStorePath()); !os.IsNotExist(err) {
		t.Fatalf("bridge plan must not create approval queue store before writeback, stat err=%v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/writeback", strings.NewReader(body))
	h.ApprovalQueueWriteback(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("writeback status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/approval-queue/bridge/plan", strings.NewReader(body))
	h.ApprovalQueueBridgePlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bridge plan after writeback status=%d body=%s", w.Code, w.Body.String())
	}
	var persisted struct {
		Plan ApprovalManagerBridgePlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&persisted); err != nil {
		t.Fatalf("decode persisted bridge plan: %v", err)
	}
	if !persisted.Plan.SourceQueueRecordPersisted || persisted.Plan.SourceApprovalQueueRecord.RequestKey != "skill-anomaly-writer-bridge-key" || persisted.Plan.GlobalApprovalEnqueueReady || persisted.Plan.MerkleAppendReady || persisted.Plan.TrustMutationReady || persisted.Plan.ActionReleaseReady {
		t.Fatalf("bridge plan should use persisted source while keeping global side effects blocked: %#v", persisted.Plan)
	}
}

func TestSkillAnomalyRejectsInvalidSkillSlug(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"../../bad","action":"read"}`))
	h.Events(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid skill slug, got %d body=%s", w.Code, w.Body.String())
	}
}
