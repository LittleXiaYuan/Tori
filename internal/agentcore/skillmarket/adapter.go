package skillmarket

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SkillSource identifies where a skill was obtained.
type SkillSource string

const (
	SourceClawHub       SkillSource = "clawhub"
	SourceToriHub       SkillSource = "torihub"
	SourceLocal         SkillSource = "local"
	SourceSelfGenerated SkillSource = "self-generated"
)

// PermLevel ranks permission levels by risk (lower = safer).
type PermLevel int

const (
	PermReadOnly PermLevel = iota // lowest risk
	PermWrite                     // medium risk
	PermNetwork                   // high risk
	PermShell                     // highest risk
)

// PermLevelFromString parses a permission name to its risk level.
func PermLevelFromString(s string) PermLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read-only", "readonly", "read":
		return PermReadOnly
	case "write", "filesystem":
		return PermWrite
	case "network", "http", "net":
		return PermNetwork
	case "shell", "exec", "command":
		return PermShell
	default:
		return PermShell // unknown = highest risk
	}
}

func (p PermLevel) String() string {
	switch p {
	case PermReadOnly:
		return "read-only"
	case PermWrite:
		return "write"
	case PermNetwork:
		return "network"
	case PermShell:
		return "shell"
	}
	return "unknown"
}

// AdaptedSkill extends SkillMeta with installation-specific fields.
type AdaptedSkill struct {
	SkillMeta
	Source        SkillSource `json:"source"`
	Slug          string      `json:"slug"`
	Content       string      `json:"content"`        // raw SKILL.md body
	Permissions   []string    `json:"permissions"`     // declared permissions
	MaxPermLevel  PermLevel   `json:"max_perm_level"`  // highest declared permission
	SecurityScore int         `json:"security_score"`  // 0-100, set by auditor
	AuditPassed   bool        `json:"audit_passed"`
}

// AdaptClawHub converts a ClawHub RemoteSkill into our internal AdaptedSkill.
func AdaptClawHub(remote RemoteSkill) (*AdaptedSkill, error) {
	if remote.Slug == "" && remote.Name == "" {
		return nil, fmt.Errorf("remote skill has no slug or name")
	}
	slug := remote.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(remote.Name, " ", "-"))
	}

	// Determine max permission level
	var maxPerm PermLevel
	for _, p := range remote.Permissions {
		level := PermLevelFromString(p)
		if level > maxPerm {
			maxPerm = level
		}
	}

	cat := CatCustom
	for _, tag := range remote.Tags {
		switch strings.ToLower(tag) {
		case "coding", "code", "dev":
			cat = CatCoding
		case "search":
			cat = CatSearch
		case "data":
			cat = CatData
		case "media", "image", "audio":
			cat = CatMedia
		case "language", "translate", "i18n":
			cat = CatLanguage
		case "education":
			cat = CatEducation
		case "productivity":
			cat = CatProductivity
		}
	}

	return &AdaptedSkill{
		SkillMeta: SkillMeta{
			Name:        remote.Name,
			Version:     remote.Version,
			Description: remote.Description,
			Author:      remote.Author,
			Category:    cat,
			Tags:        remote.Tags,
			License:     remote.License,
			Installs:    remote.Downloads,
			Rating:      remote.Rating,
			CreatedAt:   remote.CreatedAt,
			UpdatedAt:   remote.UpdatedAt,
		},
		Source:       SourceClawHub,
		Slug:         slug,
		Content:      remote.Content,
		Permissions:  remote.Permissions,
		MaxPermLevel: maxPerm,
	}, nil
}

// DependencyReport describes missing dependencies.
type DependencyReport struct {
	MissingBins []string `json:"missing_bins,omitempty"`
	MissingEnvs []string `json:"missing_envs,omitempty"`
	Satisfied   bool     `json:"satisfied"`
}

// CheckDependencies verifies that required binaries and env vars are available.
func CheckDependencies(req RemoteRequires) DependencyReport {
	report := DependencyReport{Satisfied: true}

	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			report.MissingBins = append(report.MissingBins, bin)
			report.Satisfied = false
		}
	}
	for _, env := range req.Env {
		if os.Getenv(env) == "" {
			report.MissingEnvs = append(report.MissingEnvs, env)
			report.Satisfied = false
		}
	}
	return report
}

// AdaptLocal creates an AdaptedSkill from a locally-defined skill.
func AdaptLocal(name, version, description, content string, permissions []string) *AdaptedSkill {
	var maxPerm PermLevel
	for _, p := range permissions {
		if level := PermLevelFromString(p); level > maxPerm {
			maxPerm = level
		}
	}
	return &AdaptedSkill{
		SkillMeta: SkillMeta{
			Name:        name,
			Version:     version,
			Description: description,
			Category:    CatCustom,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		Source:       SourceLocal,
		Slug:         strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Content:      content,
		Permissions:  permissions,
		MaxPermLevel: maxPerm,
	}
}
