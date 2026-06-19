package packruntime

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// PackStudioPlanRequest is the read-only request shape for Pack Studio.
// Callers can either reference an installed/catalog pack by ID or pass a
// manifest snapshot. The planner never mutates registry state or package files.
type PackStudioPlanRequest struct {
	PackID   string    `json:"pack_id,omitempty"`
	Manifest *Manifest `json:"manifest,omitempty"`
	Goal     string    `json:"goal,omitempty"`
}

// PackStudioInspectRequest asks Pack Studio to inspect a yqpack artifact
// without installing or extracting it into the runtime registry.
type PackStudioInspectRequest struct {
	PackagePath string `json:"package_path,omitempty"`
	PackageURL  string `json:"package_url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Goal        string `json:"goal,omitempty"`
}

// PackStudioWorkspaceRequest prepares an editable Pack Studio workspace from a
// yqpack artifact. It is a controlled extraction flow, not an install.
type PackStudioWorkspaceRequest struct {
	PackagePath string `json:"package_path,omitempty"`
	PackageURL  string `json:"package_url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Goal        string `json:"goal,omitempty"`
}

// PackStudioPatchRequest previews or applies a controlled text change inside a
// prepared Pack Studio workspace. It uses whole-file content replacement to
// avoid unsafe patch parser ambiguity.
type PackStudioPatchRequest struct {
	WorkspacePath string `json:"workspace_path"`
	FilePath      string `json:"file_path"`
	Content       string `json:"content"`
	Reason        string `json:"reason,omitempty"`
	Apply         bool   `json:"apply"`
}

// PackStudioRepackRequest builds a new yqpack from a prepared Pack Studio
// workspace. It does not install, enable, or execute the package.
type PackStudioRepackRequest struct {
	WorkspacePath string `json:"workspace_path"`
	OutPath       string `json:"out_path,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

// PackStudioPlanOptions carries host-derived state that is not part of a
// manifest, such as whether the pack is already installed or enabled.
type PackStudioPlanOptions struct {
	Goal      string
	Source    string
	Installed bool
	Enabled   bool
	Status    string
}

// PackStudioPlanReport is a side-effect-free plan for improving a pack. It is
// meant for UI display and model prompting, not for direct execution.
type PackStudioPlanReport struct {
	GeneratedAt   time.Time `json:"generated_at"`
	PackID        string    `json:"pack_id"`
	PackName      string    `json:"pack_name"`
	Version       string    `json:"version"`
	Source        string    `json:"source,omitempty"`
	Status        string    `json:"status,omitempty"`
	Installed     bool      `json:"installed"`
	Enabled       bool      `json:"enabled"`
	Goal          string    `json:"goal"`
	RiskLevel     string    `json:"risk_level"`
	Summary       string    `json:"summary"`
	Capabilities  []string  `json:"capabilities,omitempty"`
	Permissions   []string  `json:"permissions,omitempty"`
	FrontendPaths []string  `json:"frontend_paths,omitempty"`
	BackendRoutes []string  `json:"backend_routes,omitempty"`
	Surfaces      []string  `json:"surfaces"`
	Editable      []string  `json:"editable"`
	Guarded       []string  `json:"guarded"`
	Warnings      []string  `json:"warnings,omitempty"`
	EditableFiles []string  `json:"editable_files"`
	DiffPreview   string    `json:"diff_preview"`
	AuditSteps    []string  `json:"audit_steps"`
	PackageSteps  []string  `json:"package_steps"`
	RollbackSteps []string  `json:"rollback_steps"`
	CogniUse      []string  `json:"cogni_use"`
	XiaoyuPrompt  string    `json:"xiaoyu_prompt"`
}

// YqpackEntryReport is a safe file listing entry inside a .yqpack archive.
// It never includes file bytes.
type YqpackEntryReport struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	SizeBytes   int64  `json:"size_bytes"`
	Editable    bool   `json:"editable"`
	Reason      string `json:"reason"`
	NeedsSource bool   `json:"needs_source,omitempty"`
}

// YqpackInspectReport is a read-only archive inspection result for Pack
// Studio. It combines artifact facts, file classification, and the usual
// manifest-derived modification plan.
type YqpackInspectReport struct {
	GeneratedAt    time.Time            `json:"generated_at"`
	Source         string               `json:"source"`
	SHA256         string               `json:"sha256"`
	ExpectedSHA256 string               `json:"expected_sha256,omitempty"`
	SHA256Match    bool                 `json:"sha256_match"`
	SizeBytes      int64                `json:"size_bytes"`
	Manifest       Manifest             `json:"manifest"`
	Entries        []YqpackEntryReport  `json:"entries"`
	EntryCount     int                  `json:"entry_count"`
	EditableCount  int                  `json:"editable_count"`
	GuardedCount   int                  `json:"guarded_count"`
	Warnings       []string             `json:"warnings,omitempty"`
	Plan           PackStudioPlanReport `json:"plan"`
}

// PackStudioWorkspaceReport describes a prepared editable workspace and the
// exact commands needed to audit, repackage, and roll back safely.
type PackStudioWorkspaceReport struct {
	GeneratedAt      time.Time           `json:"generated_at"`
	WorkspacePath    string              `json:"workspace_path"`
	WorkspaceID      string              `json:"workspace_id"`
	PackageSource    string              `json:"package_source"`
	OriginalSHA256   string              `json:"original_sha256"`
	ExpectedSHA256   string              `json:"expected_sha256,omitempty"`
	SHA256Match      bool                `json:"sha256_match"`
	Manifest         Manifest            `json:"manifest"`
	Inspect          YqpackInspectReport `json:"inspect"`
	EditableFiles    []string            `json:"editable_files"`
	GuardedFiles     []string            `json:"guarded_files"`
	AuditCommands    []string            `json:"audit_commands"`
	RepackCommands   []string            `json:"repack_commands"`
	RollbackCommands []string            `json:"rollback_commands"`
	NextSteps        []string            `json:"next_steps"`
	Warnings         []string            `json:"warnings,omitempty"`
}

// PackStudioPatchReport is the preview/apply result for one controlled
// workspace text-file change.
type PackStudioPatchReport struct {
	GeneratedAt   time.Time `json:"generated_at"`
	WorkspacePath string    `json:"workspace_path"`
	FilePath      string    `json:"file_path"`
	RelativePath  string    `json:"relative_path"`
	Applied       bool      `json:"applied"`
	Reason        string    `json:"reason,omitempty"`
	OldSHA256     string    `json:"old_sha256,omitempty"`
	NewSHA256     string    `json:"new_sha256"`
	DiffPreview   string    `json:"diff_preview"`
	Warnings      []string  `json:"warnings,omitempty"`
	NextSteps     []string  `json:"next_steps"`
}

// PackStudioRepackReport describes the newly built yqpack artifact and a
// read-only inspection report for the artifact.
type PackStudioRepackReport struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	WorkspacePath string              `json:"workspace_path"`
	PackagePath   string              `json:"package_path"`
	SHA256        string              `json:"sha256"`
	SizeBytes     int64               `json:"size_bytes"`
	Manifest      Manifest            `json:"manifest"`
	Inspect       YqpackInspectReport `json:"inspect"`
	Warnings      []string            `json:"warnings,omitempty"`
	NextSteps     []string            `json:"next_steps"`
}

// BuildPackStudioPlan turns a manifest into a conservative, auditable
// improvement plan. It deliberately avoids source writes, install/enable
// changes, package extraction, signing changes, or direct model execution.
func BuildPackStudioPlan(manifest Manifest, opts PackStudioPlanOptions) PackStudioPlanReport {
	goal := strings.TrimSpace(opts.Goal)
	if goal == "" {
		goal = "让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。"
	}
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = strings.TrimSpace(manifest.Status)
	}
	if status == "" {
		if opts.Enabled {
			status = string(PackStatusEnabled)
		} else if opts.Installed {
			status = string(PackStatusInstalled)
		} else {
			status = "catalog"
		}
	}
	frontendPaths := packStudioFrontendPaths(manifest)
	backendRoutes := packStudioBackendRoutes(manifest)
	capabilities := packStudioCleanStrings(manifest.Backend.Capabilities)
	permissions := packStudioCleanStrings(manifest.Backend.Permissions)
	risk := packStudioRiskLevel(manifest)
	surfaces := packStudioSurfaces(manifest)
	editable := packStudioEditable(manifest, frontendPaths, capabilities)
	guarded := packStudioGuarded(manifest, backendRoutes, risk)
	warnings := packStudioWarnings(manifest, risk, permissions)
	editableFiles := packStudioEditableFiles(manifest, frontendPaths, backendRoutes)
	auditSteps := packStudioAuditSteps(manifest, backendRoutes)
	packageSteps := packStudioPackageSteps(manifest)
	rollbackSteps := packStudioRollbackSteps(manifest, risk)
	diffPreview := packStudioDiffPreview(manifest, goal, frontendPaths)
	cogniUse := packStudioCogniUse(manifest, capabilities, frontendPaths)

	report := PackStudioPlanReport{
		GeneratedAt:   time.Now().UTC(),
		PackID:        manifest.ID,
		PackName:      manifest.Name,
		Version:       manifest.Version,
		Source:        strings.TrimSpace(opts.Source),
		Status:        status,
		Installed:     opts.Installed,
		Enabled:       opts.Enabled,
		Goal:          goal,
		RiskLevel:     risk,
		Summary:       packStudioSummary(manifest, surfaces, risk),
		Capabilities:  capabilities,
		Permissions:   permissions,
		FrontendPaths: frontendPaths,
		BackendRoutes: backendRoutes,
		Surfaces:      surfaces,
		Editable:      editable,
		Guarded:       guarded,
		Warnings:      warnings,
		EditableFiles: editableFiles,
		DiffPreview:   diffPreview,
		AuditSteps:    auditSteps,
		PackageSteps:  packageSteps,
		RollbackSteps: rollbackSteps,
		CogniUse:      cogniUse,
	}
	report.XiaoyuPrompt = packStudioPrompt(report)
	return report
}

func packStudioSummary(manifest Manifest, surfaces []string, risk string) string {
	if len(surfaces) == 0 {
		return fmt.Sprintf("%s 是一个 manifest-only 能力包；优先补清用途、入口和可验证结果。", manifest.Name)
	}
	return fmt.Sprintf("%s 覆盖 %s；当前风险等级 %s，适合先做可感知入口、结果反馈和权限说明。", manifest.Name, strings.Join(surfaces, "/"), risk)
}

func packStudioFrontendPaths(manifest Manifest) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, menu := range manifest.Frontend.Menus {
		add(menu.Path)
	}
	for _, route := range manifest.Frontend.Routes {
		add(route.Path)
	}
	slices.Sort(paths)
	return paths
}

func packStudioBackendRoutes(manifest Manifest) []string {
	var routes []string
	for _, spec := range manifest.Backend.RouteSpecs {
		method := strings.ToUpper(strings.TrimSpace(spec.Method))
		path := strings.TrimSpace(spec.Path)
		if path == "" {
			continue
		}
		if method == "" {
			routes = append(routes, path)
		} else {
			routes = append(routes, method+" "+path)
		}
	}
	if len(routes) == 0 {
		routes = append(routes, packStudioCleanStrings(manifest.Backend.Routes)...)
	}
	slices.Sort(routes)
	return routes
}

func packStudioCleanStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func packStudioRiskLevel(manifest Manifest) string {
	joined := strings.ToLower(strings.Join(manifest.Backend.Permissions, " ") + " " + strings.Join(manifest.Backend.Capabilities, " "))
	switch {
	case strings.Contains(joined, "computer") || strings.Contains(joined, "desktop") || strings.Contains(joined, "browser:write") || strings.Contains(joined, "wasm:execute"):
		return "high"
	case strings.Contains(joined, "write") || strings.Contains(joined, "delete") || strings.Contains(joined, "network") || strings.Contains(joined, "download") || strings.Contains(joined, "admin"):
		return "medium"
	default:
		if manifest.Backend.IsWasm() {
			return "medium"
		}
		return "low"
	}
}

func packStudioSurfaces(manifest Manifest) []string {
	var surfaces []string
	if len(manifest.Frontend.Menus) > 0 || len(manifest.Frontend.Routes) > 0 {
		surfaces = append(surfaces, "frontend")
	}
	if manifest.Frontend.Assets.Type == FrontendAssetsTypeIframeBundle {
		surfaces = append(surfaces, "iframe-bundle")
	}
	if len(manifest.Backend.RouteSpecs) > 0 || len(manifest.Backend.Routes) > 0 {
		surfaces = append(surfaces, "backend")
	}
	if manifest.Backend.IsWasm() {
		surfaces = append(surfaces, "wasm")
	}
	if len(manifest.Backend.ToolSpecs) > 0 {
		surfaces = append(surfaces, "agent-tools")
	}
	if manifest.SDK.TypeScript != "" || manifest.SDK.Go != "" || manifest.SDK.Python != "" {
		surfaces = append(surfaces, "sdk")
	}
	if len(surfaces) == 0 {
		surfaces = append(surfaces, "manifest")
	}
	return surfaces
}

func packStudioEditable(manifest Manifest, frontendPaths []string, capabilities []string) []string {
	editable := []string{"用途说明、起手示例、入口文案、可用度分层和权限解释可以从 manifest/前端展示层优化。"}
	if len(frontendPaths) > 0 {
		editable = append(editable, "已有前端入口，可优先改页面文案、交互提示、空态、结果区和任务入口。")
	}
	if manifest.Frontend.Assets.Type == FrontendAssetsTypeIframeBundle {
		editable = append(editable, "这是独立界面包；若 yqpack 内含 iframe 静态资源，可在沙箱边界内优化界面。")
	}
	if manifest.Backend.IsWasm() {
		editable = append(editable, "WASM 能力可以扩展 host 调用说明、输入输出 schema 和审计提示；改二进制逻辑需要源码。")
	}
	if len(capabilities) > 0 {
		editable = append(editable, "能力声明可用于生成更清楚的 Cogni/Planner 使用说明，但第一阶段不改决策算法。")
	}
	return editable
}

func packStudioGuarded(manifest Manifest, backendRoutes []string, risk string) []string {
	guarded := []string{
		"不直接修改已签名或已安装包；先生成 diff 方案，用户确认后再打包为新 yqpack。",
		"不扩大权限、不新增高风险 route，除非用户明确授权并更新权限说明。",
	}
	if len(backendRoutes) > 0 {
		guarded = append(guarded, "后端路由逻辑属于运行时能力，改行为需要对应源码和 Go/Pack 测试。")
	}
	if manifest.Backend.IsWasm() {
		guarded = append(guarded, "不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。")
	}
	if manifest.Frontend.Assets.Type == FrontendAssetsTypeIframeBundle {
		guarded = append(guarded, "iframe 仍无宿主 token，只能调用 manifest 声明的 route。")
	}
	if risk == "high" {
		guarded = append(guarded, "高风险能力必须保留授权说明、审计线索和可回滚路径。")
	}
	return guarded
}

func packStudioWarnings(manifest Manifest, risk string, permissions []string) []string {
	var warnings []string
	stage := strings.ToLower(strings.TrimSpace(manifest.Metadata["stage"]))
	blueprint := strings.ToLower(strings.TrimSpace(manifest.Metadata["blueprint"]))
	if strings.Contains(stage, "experimental") || strings.Contains(stage, "alpha") || strings.Contains(blueprint, "experimental") {
		warnings = append(warnings, "这个包仍是实验能力，改造时不要把它包装成稳定承诺。")
	}
	if len(permissions) == 0 && (len(manifest.Backend.RouteSpecs) > 0 || manifest.Backend.IsWasm()) {
		warnings = append(warnings, "manifest 未声明权限，若新增能力必须先补权限与风险说明。")
	}
	if risk == "high" {
		warnings = append(warnings, "高风险能力需要显式授权、可撤销开关和失败后回滚路径。")
	}
	if len(packStudioFrontendPaths(manifest)) == 0 {
		warnings = append(warnings, "没有前端入口，用户可能只能看到它存在却不知道去哪使用。")
	}
	return warnings
}

func packStudioEditableFiles(manifest Manifest, frontendPaths []string, backendRoutes []string) []string {
	slug := packStudioSlug(manifest)
	files := []string{fmt.Sprintf("packs/official/%s-pack/pack.json", slug)}
	for _, path := range frontendPaths {
		if routeSlug, ok := strings.CutPrefix(path, "/packs/"); ok {
			routeSlug = strings.Trim(strings.Split(routeSlug, "?")[0], "/")
			if routeSlug != "" {
				files = append(files, fmt.Sprintf("apps/web/src/app/packs/%s/page.tsx", routeSlug))
			}
		}
	}
	if len(backendRoutes) > 0 {
		files = append(files, fmt.Sprintf("internal/packs/%s/", strings.ReplaceAll(slug, "-", "")))
	}
	files = append(files, fmt.Sprintf("apps/web/src/app/__tests__/%s-pack-page.test.tsx", slug))
	return packStudioCleanStrings(files)
}

func packStudioAuditSteps(manifest Manifest, backendRoutes []string) []string {
	steps := []string{
		"只读展开 yqpack 或源码目录，确认 pack.json、frontend、backend、sdk 文件是否齐全。",
		"检查 diff 是否扩大权限、改变签名信任、绕过 routeSpecs 或隐藏高风险动作。",
		"node scripts\\check-pack-usability.mjs --strict",
	}
	if len(backendRoutes) > 0 {
		steps = append(steps, "go test ./pkg/packruntime ./internal/controlplane/gateway ./internal/packs/... ./cmd/agent -count=1")
	}
	if manifest.Backend.IsWasm() {
		steps = append(steps, "go test ./internal/controlplane/gateway -run WASM -count=1")
	}
	steps = append(steps, "cd apps/web && npm run typecheck", "cd apps/web && npm test")
	return steps
}

func packStudioPackageSteps(manifest Manifest) []string {
	slug := packStudioSlug(manifest)
	return []string{
		fmt.Sprintf("node scripts\\release-pack.mjs --pack packs\\official\\%s-pack --dry-run", slug),
		fmt.Sprintf("go run ./cmd/yunque-plugin pack packs\\official\\%s-pack --out dist\\packs\\%s-%s.yqpack", slug, slug, manifest.Version),
		"重新计算 sha256/sizeBytes，刷新 catalog/release 元数据后再安装。",
	}
}

func packStudioRollbackSteps(manifest Manifest, risk string) []string {
	steps := []string{
		"保留原始 yqpack、原始 pack.json 和 installed registry 里的 previousVersion。",
		"新包作为 fork/local 版本安装；验证失败时禁用新版本并回滚上一版本。",
	}
	if risk == "high" {
		steps = append(steps, "如果涉及高风险权限，回滚后重新跑 backend-route-audit 和 Pack 可用性审计。")
	}
	if manifest.Update.Rollback {
		steps = append(steps, "manifest 已声明 rollback=true，可优先走 Pack Runtime 回滚入口。")
	}
	return steps
}

func packStudioDiffPreview(manifest Manifest, goal string, frontendPaths []string) string {
	slug := packStudioSlug(manifest)
	currentDescription := strings.TrimSpace(manifest.Description)
	if currentDescription == "" {
		currentDescription = "未填写用途说明"
	}
	primaryPath := "/chat"
	if len(frontendPaths) > 0 {
		primaryPath = frontendPaths[0]
	}
	return strings.Join([]string{
		fmt.Sprintf("diff --git a/packs/official/%s-pack/pack.json b/packs/official/%s-pack/pack.json", slug, slug),
		fmt.Sprintf("--- a/packs/official/%s-pack/pack.json", slug),
		fmt.Sprintf("+++ b/packs/official/%s-pack/pack.json", slug),
		"@@ metadata @@",
		fmt.Sprintf("- \"description\": %q", currentDescription),
		fmt.Sprintf("+ \"description\": %q", goal),
		fmt.Sprintf("+ \"metadata.primaryActionLabel\": %q", "打开并验证 "+manifest.Name+" 的结果"),
		fmt.Sprintf("+ \"metadata.primaryActionPath\": %q", primaryPath),
		"+ \"metadata.example1\": \"从 Chat 说明目标，让云雀调用该能力并返回可查看结果。\"",
		"+ \"metadata.example2\": \"在能力界面查看执行状态、产物、限制与下一步操作。\"",
		"+ \"metadata.limitation\": \"改包前必须经过 diff 预览、测试和重新打包，不直接修改已安装版本。\"",
		"",
		fmt.Sprintf("diff --git a/apps/web/src/app/packs/%s/page.tsx b/apps/web/src/app/packs/%s/page.tsx", slug, slug),
		fmt.Sprintf("--- a/apps/web/src/app/packs/%s/page.tsx", slug),
		fmt.Sprintf("+++ b/apps/web/src/app/packs/%s/page.tsx", slug),
		"@@ user-facing surface @@",
		"+ 增加结果区、权限说明、失败提示和回到 Chat/任务中心的入口。",
		"+ 对 WASM/iframe/后端能力保留沙箱、授权和 route 边界说明。",
	}, "\n")
}

func packStudioCogniUse(manifest Manifest, capabilities []string, frontendPaths []string) []string {
	notes := []string{"Cogni 只读取低 token 的 capability/permission/route 摘要，按需再展开完整 manifest。"}
	for _, capability := range capabilities {
		switch lower := strings.ToLower(capability); {
		case strings.Contains(lower, "memory") || strings.Contains(lower, "knowledge"):
			notes = append(notes, capability+"：适合在需要记忆、知识或长期上下文时被召回。")
		case strings.Contains(lower, "computer"):
			notes = append(notes, capability+"：启用后 Planner 可生成电脑使用计划；当前不执行本机控制。")
		case strings.Contains(lower, "browser"):
			notes = append(notes, capability+"：适合浏览器读取、检查或计划类任务；写操作需要授权。")
		case strings.Contains(lower, "wasm"):
			notes = append(notes, capability+"：适合隔离执行第三方能力，必须遵守 ABI 和权限声明。")
		default:
			notes = append(notes, capability+"：用户目标与能力说明匹配时可作为候选工具。")
		}
	}
	if len(frontendPaths) > 0 {
		notes = append(notes, "启用后可把入口提示给用户："+strings.Join(frontendPaths, ", "))
	}
	if manifest.Backend.IsWasm() {
		notes = append(notes, "WASM 包只能使用 host 允许的 ABI，不应让模型假设它能访问宿主文件或 token。")
	}
	return notes
}

func packStudioPrompt(report PackStudioPlanReport) string {
	lines := []string{
		fmt.Sprintf("请以“小羽改包”的方式改造能力包 %s (%s) v%s。", report.PackName, report.PackID, report.Version),
		"",
		"用户目标：" + report.Goal,
		"",
		"当前包信息：",
		"- 状态：" + report.Status,
		"- 来源：" + fallbackString(report.Source, "manifest/request"),
		"- 风险等级：" + report.RiskLevel,
		"- 前端入口：" + fallbackString(strings.Join(report.FrontendPaths, ", "), "无"),
		"- 后端路由：" + fallbackString(strings.Join(report.BackendRoutes, ", "), "无"),
		"- 能力声明：" + fallbackString(strings.Join(report.Capabilities, ", "), "无"),
		"- 权限声明：" + fallbackString(strings.Join(report.Permissions, ", "), "无"),
		"- 形态：" + strings.Join(report.Surfaces, ", "),
		"",
		"请按这个安全流程执行：",
		"1. 先只读检查 yqpack/pack.json/前端入口/SDK/后端 routeSpecs，列出真实可改文件。",
		"2. 明确哪些能直接改，哪些需要源码，哪些属于已编译 WASM/native Go 不能硬改。",
		"3. 先给 diff 预览和风险说明，不要直接扩大权限或绕过签名。",
		"4. 用户确认后再修改、跑测试、重新打包为新的 yqpack，并保留旧版本回滚。",
		"",
		"本包建议优先改：",
	}
	for _, item := range report.Editable {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "可改文件候选：")
	for _, item := range report.EditableFiles {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "diff 预览草案：", "```diff", report.DiffPreview, "```", "", "必须遵守：")
	for _, item := range report.Guarded {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "云雀/Cogni 如何使用它：")
	for _, item := range report.CogniUse {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "审计步骤：")
	for _, item := range report.AuditSteps {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "重新打包与回滚：")
	for _, item := range append(append([]string{}, report.PackageSteps...), report.RollbackSteps...) {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func packStudioSlug(manifest Manifest) string {
	slug := strings.TrimPrefix(strings.TrimSpace(manifest.ID), "yunque.pack.")
	if slug == "" {
		slug = strings.TrimSpace(manifest.Name)
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	replacer := strings.NewReplacer(" ", "-", "_", "-")
	slug = replacer.Replace(slug)
	if slug == "" {
		return "unknown"
	}
	return slug
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
