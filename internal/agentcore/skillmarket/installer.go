package skillmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Installer manages skill installation lifecycle with security gates.
type Installer struct {
	mu        sync.Mutex
	dataDir   string // base data dir (data/skills/)
	provider  *ClawHubProvider
	auditor   *Auditor
	market    *Market
	policy    *SecurityPolicy
	onInstall func(slug string) // callback: refresh planner prompt cache

	installed map[string]*InstalledSkill // slug → installed info
}

// InstalledSkill tracks a locally installed skill.
type InstalledSkill struct {
	Slug          string      `json:"slug"`
	Name          string      `json:"name"`
	Version       string      `json:"version"`
	Description   string      `json:"description"`
	Source        SkillSource `json:"source"`
	Permissions   []string    `json:"permissions"`
	SecurityScore int         `json:"security_score"`
	InstalledAt   time.Time   `json:"installed_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Enabled       bool        `json:"enabled"`
}

// NewInstaller creates a skill installer with all dependencies.
func NewInstaller(dataDir string, provider *ClawHubProvider, auditor *Auditor, market *Market) *Installer {
	inst := &Installer{
		dataDir:   dataDir,
		provider:  provider,
		auditor:   auditor,
		market:    market,
		installed: make(map[string]*InstalledSkill),
	}
	inst.loadInstalled()
	return inst
}

// SetOnInstall registers a callback invoked after successful installation.
func (inst *Installer) SetOnInstall(fn func(slug string)) { inst.onInstall = fn }

// SetPolicy sets the security policy for install-time checks.
func (inst *Installer) SetPolicy(p *SecurityPolicy) { inst.policy = p }

// Install performs the complete installation flow for a ClawHub skill.
// Returns the audit report for the caller to inspect.
func (inst *Installer) Install(ctx context.Context, slug string) (*AuditReport, error) {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	slog.Info("installer: starting", "slug", slug)

	// Step 1: Fetch from ClawHub
	if inst.provider == nil {
		return nil, fmt.Errorf("clawhub provider not configured")
	}
	remote, err := inst.provider.Fetch(slug)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Step 1b: Download SKILL.md content if not included in metadata
	if remote.Content == "" {
		if raw, dlErr := inst.provider.Download(slug, remote.Version); dlErr == nil && len(raw) > 0 {
			remote.Content = string(raw)
		} else {
			slog.Warn("installer: SKILL.md download empty, using description as fallback", "slug", slug, "err", dlErr)
			remote.Content = remote.Description
		}
	}

	// Step 2: Adapt to internal format
	adapted, err := AdaptClawHub(*remote)
	if err != nil {
		return nil, fmt.Errorf("adapt: %w", err)
	}

	// Step 3: Check dependencies
	depReport := CheckDependencies(remote.Requires)
	if !depReport.Satisfied {
		return nil, fmt.Errorf("unsatisfied dependencies: bins=%v envs=%v",
			depReport.MissingBins, depReport.MissingEnvs)
	}

	// Step 4: Three-layer security audit
	report := inst.auditor.Audit(ctx, adapted)
	adapted.SecurityScore = report.Score
	adapted.AuditPassed = report.Passed

	if !report.Passed {
		return report, fmt.Errorf("security audit failed: score %d/100 (min 60 required), findings: %d critical",
			report.Score, countCritical(report.Findings))
	}

	// Step 4b: Security policy enforcement
	if inst.policy != nil {
		check := inst.policy.Check(slug, remote.Author, adapted.Permissions, report.Score, true)
		if !check.Allowed {
			return report, fmt.Errorf("security policy blocked: %s", check.Reason)
		}
	}

	// Step 5: Write to disk
	if err := inst.writeToDisk(slug, adapted, report); err != nil {
		return report, fmt.Errorf("write: %w", err)
	}

	// Step 6: Register in local market
	if inst.market != nil {
		inst.market.Publish(adapted.SkillMeta)
		inst.market.RecordInstall(adapted.Name)
	}

	// Step 7: Track installation
	inst.installed[slug] = &InstalledSkill{
		Slug:          slug,
		Name:          adapted.Name,
		Version:       adapted.Version,
		Description:   adapted.Description,
		Source:        adapted.Source,
		Permissions:   adapted.Permissions,
		SecurityScore: report.Score,
		InstalledAt:   time.Now(),
		UpdatedAt:     time.Now(),
		Enabled:       true,
	}
	inst.saveInstalled()

	// Step 8: Trigger prompt refresh
	if inst.onInstall != nil {
		inst.onInstall(slug)
	}

	slog.Info("installer: complete",
		"slug", slug,
		"score", report.Score,
		"version", adapted.Version,
	)
	return report, nil
}

// InstallLocal installs a locally-defined skill (bypasses ClawHub fetch).
func (inst *Installer) InstallLocal(ctx context.Context, adapted *AdaptedSkill) (*AuditReport, error) {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	adapted.Source = SourceLocal

	// Still run security audit
	report := inst.auditor.Audit(ctx, adapted)
	adapted.SecurityScore = report.Score
	adapted.AuditPassed = report.Passed

	if !report.Passed {
		return report, fmt.Errorf("security audit failed: score %d/100", report.Score)
	}

	if err := inst.writeToDisk(adapted.Slug, adapted, report); err != nil {
		return report, fmt.Errorf("write: %w", err)
	}

	if inst.market != nil {
		inst.market.Publish(adapted.SkillMeta)
	}

	inst.installed[adapted.Slug] = &InstalledSkill{
		Slug:          adapted.Slug,
		Name:          adapted.Name,
		Version:       adapted.Version,
		Description:   adapted.Description,
		Source:        adapted.Source,
		Permissions:   adapted.Permissions,
		SecurityScore: report.Score,
		InstalledAt:   time.Now(),
		UpdatedAt:     time.Now(),
		Enabled:       true,
	}
	inst.saveInstalled()

	if inst.onInstall != nil {
		inst.onInstall(adapted.Slug)
	}
	return report, nil
}

// Uninstall removes a skill from disk and registry.
func (inst *Installer) Uninstall(slug string) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if _, ok := inst.installed[slug]; !ok {
		return fmt.Errorf("skill %q not installed", slug)
	}

	// Remove from disk
	dir := filepath.Join(inst.dataDir, slug)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("installer: remove dir failed", "slug", slug, "err", err)
	}

	// Remove from market
	if inst.market != nil {
		inst.market.Remove(slug)
	}

	delete(inst.installed, slug)
	inst.saveInstalled()

	if inst.onInstall != nil {
		inst.onInstall(slug) // refresh prompt
	}

	slog.Info("installer: uninstalled", "slug", slug)
	return nil
}

// Installed returns all currently installed skills.
func (inst *Installer) Installed() []*InstalledSkill {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	var out []*InstalledSkill
	for _, s := range inst.installed {
		copy := *s
		out = append(out, &copy)
	}
	return out
}

// IsInstalled checks if a skill is installed.
func (inst *Installer) IsInstalled(slug string) bool {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	_, ok := inst.installed[slug]
	return ok
}

// GetInstalled returns info for a single installed skill.
func (inst *Installer) GetInstalled(slug string) (*InstalledSkill, bool) {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	s, ok := inst.installed[slug]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// GetSkillContent reads the SKILL.md body for an installed skill from disk.
func (inst *Installer) GetSkillContent(slug string) (string, error) {
	inst.mu.Lock()
	_, ok := inst.installed[slug]
	inst.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("skill %q not installed", slug)
	}
	path := filepath.Join(inst.dataDir, slug, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill content: %w", err)
	}
	content := string(data)
	if content == "" {
		// Fallback to meta.json description
		metaPath := filepath.Join(inst.dataDir, slug, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err == nil {
			var meta struct {
				Description string `json:"description"`
			}
			if json.Unmarshal(metaData, &meta) == nil && meta.Description != "" {
				content = meta.Description
			}
		}
	}
	return content, nil
}

// GetAuditReport reads the audit report for an installed skill from disk.
func (inst *Installer) GetAuditReport(slug string) (*AuditReport, error) {
	inst.mu.Lock()
	_, ok := inst.installed[slug]
	inst.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("skill %q not installed", slug)
	}
	path := filepath.Join(inst.dataDir, slug, "audit_report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read audit report: %w", err)
	}
	var report AuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("decode audit report: %w", err)
	}
	return &report, nil
}

// SetEnabled enables or disables an installed skill.
func (inst *Installer) SetEnabled(slug string, enabled bool) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	s, ok := inst.installed[slug]
	if !ok {
		return fmt.Errorf("skill %q not installed", slug)
	}
	s.Enabled = enabled
	inst.saveInstalled()
	return nil
}

// ── Version management ──

// VersionInfo describes one archived version.
type VersionInfo struct {
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at,omitempty"`
	Current     bool      `json:"current"`
}

// ListVersions returns locally archived versions for a skill.
func (inst *Installer) ListVersions(slug string) ([]VersionInfo, error) {
	inst.mu.Lock()
	cur, ok := inst.installed[slug]
	inst.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("skill %q not installed", slug)
	}

	vDir := filepath.Join(inst.dataDir, slug, "versions")
	entries, err := os.ReadDir(vDir)
	if err != nil {
		return []VersionInfo{{Version: cur.Version, Current: true}}, nil
	}

	var out []VersionInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		vi := VersionInfo{Version: e.Name(), Current: e.Name() == cur.Version}
		if info, err2 := e.Info(); err2 == nil {
			vi.InstalledAt = info.ModTime()
		}
		out = append(out, vi)
	}
	if len(out) == 0 {
		return []VersionInfo{{Version: cur.Version, Current: true}}, nil
	}
	return out, nil
}

// UpdateInfo describes available update status for a skill.
type UpdateInfo struct {
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
}

// CheckUpdate queries the remote hub for a newer version of a single skill.
func (inst *Installer) CheckUpdate(ctx context.Context, slug string) (*UpdateInfo, error) {
	inst.mu.Lock()
	cur, ok := inst.installed[slug]
	inst.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("skill %q not installed", slug)
	}

	info := &UpdateInfo{
		Slug:           slug,
		Name:           cur.Name,
		CurrentVersion: cur.Version,
		LatestVersion:  cur.Version,
	}

	remote, err := inst.fetchRemote(slug)
	if err != nil {
		return info, nil // can't reach remote, no update info
	}

	info.LatestVersion = remote.Version
	info.HasUpdate = remote.Version != cur.Version
	return info, nil
}

// CheckAllUpdates checks all installed skills for available updates.
func (inst *Installer) CheckAllUpdates(ctx context.Context) []UpdateInfo {
	inst.mu.Lock()
	slugs := make([]string, 0, len(inst.installed))
	for slug := range inst.installed {
		slugs = append(slugs, slug)
	}
	inst.mu.Unlock()

	var out []UpdateInfo
	for _, slug := range slugs {
		info, err := inst.CheckUpdate(ctx, slug)
		if err == nil && info != nil {
			out = append(out, *info)
		}
	}
	return out
}

// Update re-installs a skill from the remote hub (latest version).
func (inst *Installer) Update(ctx context.Context, slug string) (*AuditReport, error) {
	// Reuse Install — it overwrites the current version and archives the new one.
	return inst.Install(ctx, slug)
}

// Rollback restores a previously archived version.
func (inst *Installer) Rollback(slug, targetVersion string) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	cur, ok := inst.installed[slug]
	if !ok {
		return fmt.Errorf("skill %q not installed", slug)
	}
	if cur.Version == targetVersion {
		return fmt.Errorf("already on version %s", targetVersion)
	}

	vDir := filepath.Join(inst.dataDir, slug, "versions", targetVersion)
	if _, err := os.Stat(vDir); os.IsNotExist(err) {
		return fmt.Errorf("version %s not archived on disk", targetVersion)
	}

	// Restore SKILL.md
	skillData, _ := os.ReadFile(filepath.Join(vDir, "SKILL.md"))
	dir := filepath.Join(inst.dataDir, slug)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), skillData, 0644)

	// Restore meta.json
	metaData, _ := os.ReadFile(filepath.Join(vDir, "meta.json"))
	os.WriteFile(filepath.Join(dir, "meta.json"), metaData, 0644)

	// Update installed record
	cur.Version = targetVersion
	cur.UpdatedAt = time.Now()

	// Try to read the archived meta for full info
	if len(metaData) > 0 {
		var meta AdaptedSkill
		if json.Unmarshal(metaData, &meta) == nil {
			if meta.Name != "" {
				cur.Name = meta.Name
			}
			if meta.Description != "" {
				cur.Description = meta.Description
			}
			if meta.Permissions != nil {
				cur.Permissions = meta.Permissions
			}
		}
	}

	inst.saveInstalled()

	if inst.onInstall != nil {
		inst.onInstall(slug)
	}

	slog.Info("installer: rollback complete", "slug", slug, "version", targetVersion)
	return nil
}

// fetchRemote tries all configured providers to fetch remote metadata.
func (inst *Installer) fetchRemote(slug string) (*RemoteSkill, error) {
	if inst.provider != nil {
		r, err := inst.provider.Fetch(slug)
		if err == nil {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no remote source for %q", slug)
}

// ── Disk operations ──

func (inst *Installer) writeToDisk(slug string, skill *AdaptedSkill, report *AuditReport) error {
	dir := filepath.Join(inst.dataDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write SKILL.md
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill.Content), 0644); err != nil {
		return err
	}

	// Write meta.json
	metaData, _ := json.MarshalIndent(skill, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaData, 0644); err != nil {
		return err
	}

	// Write audit_report.json
	reportData, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "audit_report.json"), reportData, 0644); err != nil {
		return err
	}

	// Version management: copy to versions/
	versionDir := filepath.Join(dir, "versions", skill.Version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return err
	}
	os.WriteFile(filepath.Join(versionDir, "SKILL.md"), []byte(skill.Content), 0644)
	os.WriteFile(filepath.Join(versionDir, "meta.json"), metaData, 0644)

	return nil
}

// ── Installed state persistence ──

func (inst *Installer) installedPath() string {
	return filepath.Join(inst.dataDir, "installed.json")
}

func (inst *Installer) loadInstalled() {
	data, err := os.ReadFile(inst.installedPath())
	if err != nil {
		return
	}
	var skills []*InstalledSkill
	if err := json.Unmarshal(data, &skills); err != nil {
		slog.Warn("installer: load installed failed", "err", err)
		return
	}
	for _, s := range skills {
		inst.installed[s.Slug] = s
	}
}

func (inst *Installer) saveInstalled() {
	var skills []*InstalledSkill
	for _, s := range inst.installed {
		skills = append(skills, s)
	}
	data, _ := json.MarshalIndent(skills, "", "  ")
	os.MkdirAll(inst.dataDir, 0755)
	if err := os.WriteFile(inst.installedPath(), data, 0644); err != nil {
		slog.Warn("installer: save installed failed", "err", err)
	}
}

func countCritical(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == SevCritical {
			n++
		}
	}
	return n
}
