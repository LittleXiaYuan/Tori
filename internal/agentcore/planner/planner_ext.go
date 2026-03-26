package planner

// SetAckEnabled enables or disables the acknowledgement feature.
func (p *Planner) SetAckEnabled(enabled bool) {
	p.ackEnabled = enabled
}

// SetLocale sets the agent locale (e.g. "zh-CN", "en-US").
func (p *Planner) SetLocale(locale string) {
	p.locale = locale
}
