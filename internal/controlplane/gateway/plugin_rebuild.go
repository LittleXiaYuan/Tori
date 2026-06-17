package gateway

// rebuildSkillsFromPlugins rebuilds the skill registry and planner domain prompt.
// Uses ReplaceAll for atomicity: the registry is never observably empty, which
// matters because request handlers iterate All()/Get() concurrently. The
// skillFileLoader is run after the replace so file-sourced skills layer on top
// of plugin-sourced ones via Register().
func (g *Gateway) rebuildSkillsFromPlugins() {
	g.registry.ReplaceAll(g.pluginReg.AllSkills())
	if g.skillFileLoader != nil {
		g.skillFileLoader.LoadAll()
	}
	g.planner.SetDomainPrompt(g.pluginReg.CombinedPrompt())
}
