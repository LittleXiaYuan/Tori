package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/apperror"
)

// handleSkillHubSearch combines local market and ClawHub remote search.
func (g *Gateway) handleSkillHubSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query().Get("q")
	if query == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "query parameter 'q' required")
		return
	}

	type searchResult struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Version     string  `json:"version"`
		Author      string  `json:"author"`
		Rating      float64 `json:"rating"`
		Source      string  `json:"source"` // "local" or "clawhub"
		Installed   bool    `json:"installed"`
	}
	var results []searchResult

	// Search local market
	if g.skillMarket != nil {
		for _, s := range g.skillMarket.Search(query) {
			installed := false
			if g.skillInstaller != nil {
				installed = g.skillInstaller.IsInstalled(s.Name)
			}
			results = append(results, searchResult{
				Name:        s.Name,
				Description: s.Description,
				Version:     s.Version,
				Author:      s.Author,
				Rating:      s.Rating,
				Source:      "local",
				Installed:   installed,
			})
		}
	}

	// Search remote hubs (ClawHub + ToriHub)
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}
	source := r.URL.Query().Get("source") // "clawhub", "torihub", or "" for all

	type hubEntry struct {
		provider skillmarket.HubProvider
		name     string
	}
	var hubs []hubEntry
	if g.clawHub != nil && (source == "" || source == "clawhub") {
		hubs = append(hubs, hubEntry{g.clawHub, "clawhub"})
	}
	if g.toriHub != nil && (source == "" || source == "torihub") {
		hubs = append(hubs, hubEntry{g.toriHub, "torihub"})
	}

	for _, hub := range hubs {
		remote, err := hub.provider.Search(query, limit)
		if err != nil {
			continue
		}
		for _, rs := range remote {
			installed := false
			if g.skillInstaller != nil {
				installed = g.skillInstaller.IsInstalled(rs.Slug)
			}
			results = append(results, searchResult{
				Name:        rs.Name,
				Description: rs.Description,
				Version:     rs.Version,
				Author:      rs.Author,
				Rating:      rs.Rating,
				Source:      hub.name,
				Installed:   installed,
			})
		}
	}

	if results == nil {
		results = []searchResult{}
	}
	json.NewEncoder(w).Encode(map[string]any{"results": results, "count": len(results)})
}

// handleSkillHubInstall installs a skill from ClawHub through the 3-layer security audit.
func (g *Gateway) handleSkillHubInstall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	if g.skillInstaller == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill installer not configured")
		return
	}

	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug is required")
		return
	}

	report, err := g.skillInstaller.Install(r.Context(), req.Slug)
	if err != nil {
		status := map[string]any{"error": err.Error()}
		if report != nil {
			status["report"] = report
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(status)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status": "installed",
		"slug":   req.Slug,
		"report": report,
	})
}

// handleSkillHubInstalled returns all installed skills.
func (g *Gateway) handleSkillHubInstalled(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillInstaller == nil {
		json.NewEncoder(w).Encode(map[string]any{"skills": []any{}, "count": 0})
		return
	}
	installed := g.skillInstaller.Installed()
	json.NewEncoder(w).Encode(map[string]any{"skills": installed, "count": len(installed)})
}

// handleSkillHubUninstall removes an installed skill.
func (g *Gateway) handleSkillHubUninstall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE or POST required")
		return
	}
	if g.skillInstaller == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill installer not configured")
		return
	}

	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug is required")
		return
	}

	if err := g.skillInstaller.Uninstall(req.Slug); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "uninstalled", "slug": req.Slug})
}

// handleSkillHubTrending returns trending skills from all configured hubs.
func (g *Gateway) handleSkillHubTrending(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}
	source := r.URL.Query().Get("source") // "clawhub", "torihub", or "" for all

	type trendingItem struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Version     string  `json:"version"`
		Author      string  `json:"author"`
		Rating      float64 `json:"rating"`
		Source      string  `json:"source"`
		Installed   bool    `json:"installed"`
	}
	var all []trendingItem

	type hubEntry struct {
		provider skillmarket.HubProvider
		name     string
	}
	var hubs []hubEntry
	if g.clawHub != nil && (source == "" || source == "clawhub") {
		hubs = append(hubs, hubEntry{g.clawHub, "clawhub"})
	}
	if g.toriHub != nil && (source == "" || source == "torihub") {
		hubs = append(hubs, hubEntry{g.toriHub, "torihub"})
	}

	cursor := r.URL.Query().Get("cursor")
	var nextCursor string

	for _, hub := range hubs {
		// Use paged API for ClawHub
		if ch, ok := hub.provider.(*skillmarket.ClawHubProvider); ok {
			result, err := ch.TrendingPaged(limit, cursor)
			if err != nil {
				continue
			}
			if result.NextCursor != "" {
				nextCursor = result.NextCursor
			}
			for _, s := range result.Skills {
				installed := false
				if g.skillInstaller != nil {
					installed = g.skillInstaller.IsInstalled(s.Slug)
				}
				all = append(all, trendingItem{
					Name:        s.Name,
					Description: s.Description,
					Version:     s.Version,
					Author:      s.Author,
					Rating:      s.Rating,
					Source:      hub.name,
					Installed:   installed,
				})
			}
			continue
		}
		// Fallback for other providers (ToriHub etc.)
		trending, err := hub.provider.Trending(limit)
		if err != nil {
			continue
		}
		for _, s := range trending {
			installed := false
			if g.skillInstaller != nil {
				installed = g.skillInstaller.IsInstalled(s.Slug)
			}
			all = append(all, trendingItem{
				Name:        s.Name,
				Description: s.Description,
				Version:     s.Version,
				Author:      s.Author,
				Rating:      s.Rating,
				Source:      hub.name,
				Installed:   installed,
			})
		}
	}

	if len(all) == 0 {
		all = []trendingItem{} // ensure JSON array, not null
	}
	resp := map[string]any{"skills": all, "count": len(all)}
	if nextCursor != "" {
		resp["next_cursor"] = nextCursor
	}
	json.NewEncoder(w).Encode(resp)
}

// handleSkillHubDetail returns comprehensive info for a single skill.
func (g *Gateway) handleSkillHubDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug parameter required")
		return
	}

	type detailResp struct {
		Slug          string                   `json:"slug"`
		Name          string                   `json:"name"`
		Description   string                   `json:"description"`
		Version       string                   `json:"version"`
		Author        string                   `json:"author"`
		Rating        float64                  `json:"rating"`
		RatingCount   int                      `json:"rating_count"`
		Installs      int64                    `json:"installs"`
		Category      string                   `json:"category"`
		Tags          []string                 `json:"tags"`
		License       string                   `json:"license"`
		Installed     bool                     `json:"installed"`
		Source        string                   `json:"source"`
		Permissions   []string                 `json:"permissions,omitempty"`
		SecurityScore int                      `json:"security_score"`
		AuditReport   *skillmarket.AuditReport `json:"audit_report,omitempty"`
		Content       string                   `json:"content,omitempty"` // SKILL.md body
		InstalledAt   string                   `json:"installed_at,omitempty"`
		UpdatedAt     string                   `json:"updated_at,omitempty"`
	}

	var resp detailResp
	resp.Slug = slug

	// Check installed status
	if g.skillInstaller != nil {
		if inst, ok := g.skillInstaller.GetInstalled(slug); ok {
			resp.Installed = true
			resp.Name = inst.Name
			resp.Description = inst.Description
			resp.Version = inst.Version
			resp.Source = string(inst.Source)
			resp.Permissions = inst.Permissions
			resp.SecurityScore = inst.SecurityScore
			resp.InstalledAt = inst.InstalledAt.Format("2006-01-02T15:04:05Z")
			resp.UpdatedAt = inst.UpdatedAt.Format("2006-01-02T15:04:05Z")
			if content, err := g.skillInstaller.GetSkillContent(slug); err == nil {
				resp.Content = content
			}
			if report, err := g.skillInstaller.GetAuditReport(slug); err == nil {
				resp.AuditReport = report
			}
		}
	}

	// Enrich from local market if available
	if g.skillMarket != nil {
		if meta, ok := g.skillMarket.Get(slug); ok {
			if resp.Name == "" {
				resp.Name = meta.Name
			}
			resp.Rating = meta.Rating
			resp.RatingCount = meta.RatingCount
			resp.Installs = meta.Installs
			resp.Category = string(meta.Category)
			resp.Tags = meta.Tags
			resp.License = meta.License
			resp.Author = meta.Author
		}
	}

	// Try remote fetch if not enough local info
	if resp.Name == "" {
		if g.clawHub != nil {
			if remote, err := g.clawHub.Fetch(slug); err == nil {
				resp.Name = remote.Name
				resp.Description = remote.Description
				resp.Version = remote.Version
				resp.Author = remote.Author
				resp.Rating = remote.Rating
				resp.Source = "clawhub"
				resp.Permissions = remote.Permissions
			}
		}
		if resp.Name == "" && g.toriHub != nil {
			if remote, err := g.toriHub.Fetch(slug); err == nil {
				resp.Name = remote.Name
				resp.Description = remote.Description
				resp.Version = remote.Version
				resp.Author = remote.Author
				resp.Rating = remote.Rating
				resp.Source = "torihub"
				resp.Permissions = remote.Permissions
			}
		}
	}

	if resp.Name == "" {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	json.NewEncoder(w).Encode(resp)
}

// handleSkillHubCheckUpdates checks all installed skills for available updates.
func (g *Gateway) handleSkillHubCheckUpdates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillInstaller == nil {
		json.NewEncoder(w).Encode(map[string]any{"updates": []any{}})
		return
	}
	updates := g.skillInstaller.CheckAllUpdates(r.Context())
	if updates == nil {
		updates = []skillmarket.UpdateInfo{}
	}
	json.NewEncoder(w).Encode(map[string]any{"updates": updates})
}

// handleSkillHubUpdate re-installs a skill from the remote hub (latest version).
func (g *Gateway) handleSkillHubUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillInstaller == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "installer not configured")
		return
	}
	var body struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug required")
		return
	}
	report, err := g.skillInstaller.Update(r.Context(), body.Slug)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "report": report})
}

// handleSkillHubRollback restores a previously archived version.
func (g *Gateway) handleSkillHubRollback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillInstaller == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "installer not configured")
		return
	}
	var body struct {
		Slug    string `json:"slug"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Slug == "" || body.Version == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug and version required")
		return
	}
	if err := g.skillInstaller.Rollback(body.Slug, body.Version); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleSkillHubVersions lists archived versions for a skill.
func (g *Gateway) handleSkillHubVersions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillInstaller == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "installer not configured")
		return
	}
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug required")
		return
	}
	versions, err := g.skillInstaller.ListVersions(slug)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"versions": versions})
}

// handleSkillHubPolicy GET returns current policy, POST updates it.
func (g *Gateway) handleSkillHubPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillPolicy == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "security policy not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(g.skillPolicy.Get())
	case http.MethodPost, http.MethodPut:
		var data skillmarket.PolicyData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid policy data")
			return
		}
		g.skillPolicy.Update(data)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "GET or POST required")
	}
}

// handleSkillHubPolicyCheck runs a dry-run policy check for a skill slug.
func (g *Gateway) handleSkillHubPolicyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillPolicy == nil {
		json.NewEncoder(w).Encode(skillmarket.PolicyCheckResult{Allowed: true})
		return
	}
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "slug required")
		return
	}

	// Gather skill metadata for policy check
	var author string
	var perms []string
	var score int
	var auditAvailable bool

	if g.skillInstaller != nil {
		if info, ok := g.skillInstaller.GetInstalled(slug); ok {
			author = ""
			perms = info.Permissions
			score = info.SecurityScore
			auditAvailable = true
		}
	}
	if g.clawHub != nil && author == "" {
		if remote, err := g.clawHub.Fetch(slug); err == nil {
			author = remote.Author
			perms = remote.Permissions
		}
	}

	result := g.skillPolicy.Check(slug, author, perms, score, auditAvailable)
	json.NewEncoder(w).Encode(result)
}

// handleSkillHubAnalytics returns comprehensive marketplace analytics.
func (g *Gateway) handleSkillHubAnalytics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type skillSummary struct {
		Slug          string  `json:"slug"`
		Name          string  `json:"name"`
		Author        string  `json:"author"`
		Version       string  `json:"version"`
		Installs      int64   `json:"installs"`
		Rating        float64 `json:"rating"`
		SecurityScore int     `json:"security_score"`
		Enabled       bool    `json:"enabled"`
	}

	resp := map[string]any{
		"total_skills":    0,
		"installed_count": 0,
		"total_installs":  int64(0),
		"avg_score":       0.0,
		"categories":      map[string]int{},
		"top_installed":   []skillSummary{},
		"top_rated":       []skillSummary{},
		"security_stats":  map[string]int{"high": 0, "medium": 0, "low": 0},
	}

	// Market stats
	if g.skillMarket != nil {
		stats := g.skillMarket.Stats()
		resp["total_skills"] = stats["total"]
		resp["total_installs"] = stats["total_installs"]
		if cats, ok := stats["categories"].(map[skillmarket.Category]int); ok {
			catMap := make(map[string]int)
			for k, v := range cats {
				catMap[string(k)] = v
			}
			resp["categories"] = catMap
		}

		// Top rated
		topRated := g.skillMarket.TopRated(10)
		var rated []skillSummary
		for _, s := range topRated {
			rated = append(rated, skillSummary{
				Slug: s.Name, Name: s.Name, Author: s.Author,
				Version: s.Version, Installs: s.Installs, Rating: s.Rating,
			})
		}
		resp["top_rated"] = rated

		// Most popular
		topPop := g.skillMarket.MostPopular(10)
		var popular []skillSummary
		for _, s := range topPop {
			popular = append(popular, skillSummary{
				Slug: s.Name, Name: s.Name, Author: s.Author,
				Version: s.Version, Installs: s.Installs, Rating: s.Rating,
			})
		}
		resp["top_installed"] = popular
	}

	// Installed skill analytics
	if g.skillInstaller != nil {
		installed := g.skillInstaller.Installed()
		resp["installed_count"] = len(installed)

		var totalScore int
		scored := 0
		secStats := map[string]int{"high": 0, "medium": 0, "low": 0}
		for _, s := range installed {
			if s.SecurityScore > 0 {
				totalScore += s.SecurityScore
				scored++
				if s.SecurityScore >= 80 {
					secStats["high"]++
				} else if s.SecurityScore >= 60 {
					secStats["medium"]++
				} else {
					secStats["low"]++
				}
			}
		}
		if scored > 0 {
			resp["avg_score"] = float64(totalScore) / float64(scored)
		}
		resp["security_stats"] = secStats
	}

	json.NewEncoder(w).Encode(resp)
}
