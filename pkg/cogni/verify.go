package cogni

import (
	"fmt"
	"strings"
)

// CheckResult is the outcome of running one ActivationCheck.
type CheckResult struct {
	CogniID    string `json:"cogni_id"`
	CheckName  string `json:"check_name,omitempty"`
	CheckIndex int    `json:"check_index"`
	Passed     bool   `json:"passed"`
	// Failure details (empty when Passed).
	Reason    string  `json:"reason,omitempty"`
	GotActive bool    `json:"got_active"`
	GotScore  float64 `json:"got_score"`
}

// VerifyDeclaration runs every ActivationCheck attached to `d`. Returns one
// CheckResult per check — cogni with no checks returns an empty slice.
//
// The evaluator is supplied by the caller so repeated calls can share a
// compiled-regex cache. Pass nil to let VerifyDeclaration allocate one.
func VerifyDeclaration(d *Declaration, eval *Evaluator) []CheckResult {
	if d == nil || len(d.Checks) == 0 {
		return nil
	}
	if eval == nil {
		eval = NewEvaluator()
	}
	out := make([]CheckResult, 0, len(d.Checks))
	for i, chk := range d.Checks {
		out = append(out, runCheck(d, chk, i, eval))
	}
	return out
}

func runCheck(d *Declaration, chk ActivationCheck, idx int, eval *Evaluator) CheckResult {
	res := CheckResult{
		CogniID:    d.ID,
		CheckName:  chk.Name,
		CheckIndex: idx,
	}

	session := Session{
		Message:       chk.Message,
		TenantID:      chk.Tenant,
		Channel:       chk.Channel,
		PriorHandover: chk.PriorHandover,
	}
	acts := eval.Evaluate([]*Declaration{d}, session)
	// Apply exclusivity to mirror runtime behaviour; since we only evaluate
	// one declaration the outcome is the same, but preserving the pipeline
	// keeps assertions honest if the author later groups cognis together
	// and reruns the same checks.
	acts = ApplyExclusivity(acts)
	if len(acts) == 0 {
		res.Reason = "evaluator returned no activations"
		return res
	}
	a := acts[0]
	res.GotActive = a.Activated
	res.GotScore = round3(a.Score)

	var failures []string

	if chk.ExpectActive != nil && a.Activated != *chk.ExpectActive {
		failures = append(failures, fmt.Sprintf("expect_active=%v got %v", *chk.ExpectActive, a.Activated))
	}
	if chk.ExpectScoreAtLeast > 0 && a.Score < chk.ExpectScoreAtLeast {
		failures = append(failures, fmt.Sprintf("expect_score_at_least=%.3f got %.3f", chk.ExpectScoreAtLeast, a.Score))
	}
	for _, want := range chk.ExpectReasonContains {
		if !reasonContains(a.Reasons, want) {
			failures = append(failures, fmt.Sprintf("reason_contains=%q not found in %v", want, a.Reasons))
		}
	}

	// No assertion set = misconfigured check, but don't blow up;
	// mark as passed but include a hint.
	if chk.ExpectActive == nil && chk.ExpectScoreAtLeast == 0 && len(chk.ExpectReasonContains) == 0 {
		res.Passed = true
		res.Reason = "no assertion configured (ignored)"
		return res
	}

	if len(failures) == 0 {
		res.Passed = true
		return res
	}
	res.Reason = strings.Join(failures, "; ")
	return res
}

func reasonContains(reasons []string, want string) bool {
	for _, r := range reasons {
		if strings.Contains(r, want) {
			return true
		}
	}
	return false
}

// VerifyAll runs verification across every registered declaration. The
// result groups per-cogni; an entry with no Results means the cogni has
// no checks.
func (r *Registry) VerifyAll() map[string][]CheckResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	eval := NewEvaluator()
	out := make(map[string][]CheckResult, len(r.entries))
	for id, e := range r.entries {
		if len(e.decl.Checks) == 0 {
			continue
		}
		out[id] = VerifyDeclaration(e.decl, eval)
	}
	return out
}

// FailedChecks walks the VerifyAll map and returns a flat list of failed
// results ordered by cogni id then check index.
func FailedChecks(results map[string][]CheckResult) []CheckResult {
	var out []CheckResult
	ids := make([]string, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	// Sort lazily; keep deterministic UI output
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	for _, id := range ids {
		for _, r := range results[id] {
			if !r.Passed && r.Reason != "no assertion configured (ignored)" {
				out = append(out, r)
			}
		}
	}
	return out
}
