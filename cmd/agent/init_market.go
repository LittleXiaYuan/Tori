package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/skillgrowth"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
)

// initMarketplace initializes the SkillHub marketplace: ClawHub + ToriHub + GitHub providers,
// installer with security audit, and hub skill tools.
func initMarketplace(app *agentrt.App, gw *gateway.Gateway, p *planner.Planner) *skillmarket.Installer {
	market := skillmarket.NewMarket(appdir.DataDir())
	_ = market.LoadFrom(appdir.File("market.json"))
	gw.SetSkillMarket(market)

	clawHub := skillmarket.NewClawHubProvider(os.Getenv("CLAWHUB_BASE_URL"), appdir.Sub("cache", "clawhub"))
	gw.SetClawHubProvider(clawHub)
	toriHub := skillmarket.NewToriHubProvider(os.Getenv("TORIHUB_BASE_URL"))
	gw.SetToriHubProvider(toriHub)

	skillAuditor := skillmarket.NewAuditor(appdir.Sub("skills"))
	installer := skillmarket.NewInstaller(appdir.Sub("skills"), clawHub, skillAuditor, market)
	installer.SetOnInstall(func(slug string) {
		p.InvalidatePromptCache()
		slog.Info("skill installed, prompt cache invalidated", "slug", slug)
	})
	installer.SetOnRegister(func(slug, name, description, content string) error {
		if content == "" {
			return nil
		}
		app.SkillRegistry.Register(&marketplaceSkill{
			slug: slug, name: name, description: description,
			content: content, llmCall: app.LLMBreaker.Call,
		})
		slog.Info("marketplace skill registered", "slug", slug, "name", name)
		return nil
	})
	skillPolicy := skillmarket.NewSecurityPolicy(appdir.File("skills", "policy.json"))
	installer.SetPolicy(skillPolicy)

	githubToken := os.Getenv("GITHUB_TOKEN")
	installer.SetGitHubProvider(skillmarket.NewGitHubSkillProvider(githubToken))
	slog.Info("github skill provider registered", "authenticated", githubToken != "")

	gw.SetSkillInstaller(installer)
	gw.SetSkillPolicy(skillPolicy)

	p.SetSkillIndex(func() []planner.SkillIndexEntry {
		installed := installer.Installed()
		entries := make([]planner.SkillIndexEntry, 0, len(installed))
		for _, s := range installed {
			if s.Enabled {
				entries = append(entries, planner.SkillIndexEntry{Slug: s.Slug, Description: s.Description})
			}
		}
		return entries
	})

	app.SkillRegistry.Register(&useSkillTool{installer: installer})
	app.SkillRegistry.Register(&searchSkillsTool{provider: clawHub})
	app.SkillRegistry.Register(&installSkillTool{installer: installer})

	// Register generate_skill tool for on-demand skill creation.
	// Skills are saved to data/skills/<slug>/ as SKILL.md + meta.json + scripts.
	skillsDir := appdir.Sub("skills")
	var skillFileLoader *skillmarket.SkillFileLoader
	if sfl, ok := app.Get("skill_file_loader"); ok {
		skillFileLoader, _ = sfl.(*skillmarket.SkillFileLoader)
	}
	genTool := &generateSkillTool{
		llmCall:  app.LLMBreaker.Call,
		skillDir: skillsDir,
		onReload: func() {
			if skillFileLoader != nil {
				skillFileLoader.LoadAll()
			}
			p.InvalidatePromptCache()
		},
	}
	app.SkillRegistry.Register(genTool)
	p.InvalidatePromptCache()

	// Autonomous skill growth: search → install → retry on missing skill
	sgCfg := planner.DefaultSkillGrowthConfig()
	if os.Getenv("SKILL_GROWTH") == "false" {
		sgCfg.Enabled = false
	}
	if os.Getenv("SKILL_GROWTH_AUTO") == "true" {
		sgCfg.AutoInstall = true
	}
	sg := planner.NewSkillGrowth(sgCfg)
	sg.SetSearch(func(ctx context.Context, query string) (string, string, bool) {
		results, err := clawHub.Search(query, 3)
		if err != nil || len(results) == 0 {
			toriResults, tErr := toriHub.Search(query, 3)
			if tErr != nil || len(toriResults) == 0 {
				return "", "", false
			}
			return toriResults[0].Slug, toriResults[0].Description, true
		}
		return results[0].Slug, results[0].Description, true
	})
	sg.SetInstall(func(ctx context.Context, slug string) (string, error) {
		_, err := installer.Install(ctx, slug)
		if err != nil {
			return "", err
		}
		// OnRegister registers the skill under marketplaceSkill.Name() (the name field),
		// which may differ from the slug. Scan registry to return the actual key.
		for _, sk := range app.SkillRegistry.All() {
			if ms, ok := sk.(*marketplaceSkill); ok && ms.slug == slug {
				return ms.Name(), nil
			}
		}
		return slug, nil
	})
	// Phase 2: skill generation from authoritative web sources (requires WebSearch + LLM)
	if searchRegRaw, ok := app.Get(agentrt.CompSearchReg); ok {
		if searchReg, ok := searchRegRaw.(*websearch.Registry); ok && len(searchReg.List()) > 0 {
			gen := planner.NewSkillGenerator(30 * time.Second)
			gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]planner.WebSearchResult, error) {
				results, err := searchReg.Search(ctx, query, limit)
				if err != nil {
					return nil, err
				}
				out := make([]planner.WebSearchResult, len(results))
				for i, r := range results {
					out[i] = planner.WebSearchResult{Title: r.Title, URL: r.URL, Snippet: r.Snippet}
				}
				return out, nil
			})
			gen.SetLLMCall(app.LLMBreaker.Call)
			gen.SetRegister(func(slug, name, description, content string) (string, error) {
				app.SkillRegistry.Register(&marketplaceSkill{
					slug: slug, name: name, description: description,
					content: content, llmCall: app.LLMBreaker.Call,
				})
				slog.Info("skill_generator: registered generated skill", "slug", slug, "name", name)
				return name, nil
			})
			gen.SetRegisterPackage(func(slug, name, description string, files []planner.SkillFile) (string, error) {
				pkgDir := filepath.Join(appdir.Sub("skills"), slug)
				_ = os.MkdirAll(pkgDir, 0755)
				var skillContent string
				for _, f := range files {
					fPath := filepath.Join(pkgDir, f.Path)
					_ = os.MkdirAll(filepath.Dir(fPath), 0755)
					_ = os.WriteFile(fPath, []byte(f.Content), 0644)
					if f.Path == "SKILL.md" {
						skillContent = f.Content
					}
				}
				if skillContent == "" && len(files) > 0 {
					skillContent = files[0].Content
				}
				app.SkillRegistry.Register(&marketplaceSkill{
					slug: slug, name: name, description: description,
					content: skillContent, llmCall: app.LLMBreaker.Call,
				})
				slog.Info("skill_generator: registered multi-file skill", "slug", slug, "files", len(files), "dir", pkgDir)
				return name, nil
			})
			sg.SetGenerate(gen.Generate)
			pipe := skillgrowth.NewPipeline(skillgrowth.DefaultPipelineConfig())
			pipe.SetGenerator(planner.NewSkillGrowthPipelineGenerator(gen.Generate))
			gw.SetSkillGrowthPipeline(pipe)
			app.Set(agentrt.CompSkillGrowthPipeline, pipe)

			// Wire auto-generation to the skill growth detector
			if detRaw, ok := app.Get("skillgrow_detector"); ok {
				if det, ok := detRaw.(*adapter.Detector); ok {
					// Compatibility fallback: the detector now emits canonical
					// skillgrowth.Gap events into the pipeline via Gateway.SetSkillGrow.
					// Keep direct generation available only when no pipeline was
					// registered.
					if _, ok := app.Get(agentrt.CompSkillGrowthPipeline); !ok {
						det.SetGenerateSkill(gen.Generate)
						slog.Info("skillgrow: direct auto-generate wired to detector")
					}
				}
			}

			slog.Info("skill generator enabled (web search → LLM → register)")
		}
	}

	p.SetSkillGrowth(sg)
	slog.Info("skill growth initialized", "enabled", sgCfg.Enabled, "auto_install", sgCfg.AutoInstall)

	return installer
}
