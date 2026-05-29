package gateway

import (
	"context"
	"strings"
	"time"

	"yunque-agent/pkg/packruntime"
)

// evolutionPackID is the pack whose enabled-state gates conversation-driven
// evolution (post-turn reflection → experience → strategy, and memory
// writeback). It is installed enabled by default, so learning is on out of the
// box; disabling it stops the agent from learning from conversations.
const evolutionPackID = "yunque.pack.inner-life"

// reflectionSem bounds how many post-conversation reflections run at once.
// Each reflection makes an LLM call, so a burst of turns must not spawn
// unbounded goroutines. At capacity, a round's reflection is skipped (the
// next turn will reflect) — mirroring the kernel's own drop-on-full policy.
var reflectionSem = make(chan struct{}, 4)

// evolutionEnabled reports whether conversation-driven evolution should run.
// With no pack runtime, or when the evolution pack isn't installed, it returns
// true so lean configurations still learn. When the pack is installed, its
// enabled-state controls the behavior — this is how "toggling the pack changes
// whether the agent evolves" works without the pack touching core control flow.
func (g *Gateway) evolutionEnabled() bool {
	if g.packRegistry == nil {
		return true
	}
	pack, ok := g.packRegistry.Get(evolutionPackID)
	if !ok {
		return true
	}
	return pack.Status == packruntime.PackStatusEnabled
}

// fireReflection runs the post-conversation reflective loop asynchronously so
// it never blocks the user's reply. This is the ignition the main chat path
// was missing: it activates the already-wired reflect → experience → strategy
// → prompt-injection loop. Gated by evolutionEnabled so the evolution pack can
// turn it off.
func (g *Gateway) fireReflection(tenantID, sessionID, intent, reply string, skillsUsed []string, modelTier string) {
	if g.reflectiveLoop == nil || strings.TrimSpace(reply) == "" || strings.TrimSpace(intent) == "" {
		return
	}
	if !g.evolutionEnabled() {
		return
	}
	select {
	case reflectionSem <- struct{}{}:
	default:
		return // at capacity; skip this round
	}
	go func() {
		defer func() { <-reflectionSem }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		g.reflectiveLoop.ReflectAndLearn(ctx, tenantID, sessionID, intent, reply, skillsUsed, modelTier)
	}()
}
