package planner

// SetAckEnabled enables or disables the acknowledgement feature.
func (p *Planner) SetAckEnabled(enabled bool) {
	executionRuntime := p.ensureExecutionRuntime()
	executionRuntime.SetAckEnabled(enabled)
}

// SetLocale sets the agent locale (e.g. "zh-CN", "en-US").
func (p *Planner) SetLocale(locale string) {
	promptRuntime := p.ensurePromptRuntime()
	promptRuntime.SetLocale(locale)
}
