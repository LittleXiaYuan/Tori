package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/router"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/safego"
)

// estimateTokens estimates token count for a string using rune-level heuristic.
// Mixed CJK/EN text averages ~3 chars per token; CJK-heavy text ~1.5, pure
// English ~4.  We walk the runes once and weight CJK characters higher.
func estimateTokens(s string) int64 {
	var cjk, other int
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			cjk++
		} else {
			other++
		}
	}
	// ~1.5 runes/token for CJK, ~4 runes/token for other
	tokens := cjk*10/15 + other/4 + 4 // +4 accounts for message formatting overhead
	if tokens < 1 {
		tokens = 1
	}
	return int64(tokens)
}

// estimateMsgTokens estimates total input tokens across all messages.
func estimateMsgTokens(msgs []llm.Message) int64 {
	var total int64
	for _, m := range msgs {
		total += estimateTokens(m.Content)
		total += 4 // per-message overhead (role, name, delimiters)
		for _, p := range m.ContentParts {
			total += estimateTokens(p.Text)
		}
	}
	total += 3 // conversation-level overhead
	return total
}

// resolveThinkingLevel determines the model tier based on thinking level.
func (g *Gateway) resolveThinkingLevel(ctx context.Context, level string, msgs []llm.Message, span *observe.Span) (tier, modelID string) {
	if level == "" {
		level = os.Getenv("THINKING_LEVEL")
	}
	switch level {
	case "deep":
		span.Attrs["thinking_level"] = "deep"
		return "expert", ""
	case "none":
		span.Attrs["thinking_level"] = "none"
		return "fast", ""
	default:
		span.Attrs["thinking_level"] = "auto"
		if g.smartRouter != nil && len(msgs) > 0 {
			routedModel, t := g.smartRouter.Route(ctx, msgs[len(msgs)-1].Content, false)
			if routedModel != nil {
				modelID = routedModel.ModelID
			}
			return t.String(), modelID
		}
		return "", ""
	}
}

// analyzeEmotion runs async emotion analysis on the last user message.
func (g *Gateway) analyzeEmotion(ctx context.Context, sessionID string, msgs []llm.Message) *emotion.Result {
	featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer == nil || !g.emotionAnalyzer.Enabled() || !featureOK || len(msgs) == 0 {
		return nil
	}
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != "user" || lastMsg.Content == "" {
		return nil
	}

	emotionCh := make(chan *emotion.Result, 1)
	emotionCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	safego.Go("chat-emotion-analyze", func() {
		hint, _ := g.emotionAnalyzer.AnalyzeText(emotionCtx, lastMsg.Content)
		emotionCh <- hint
	})

	var hint *emotion.Result
	select {
	case hint = <-emotionCh:
	case <-emotionCtx.Done():
		slog.Debug("chat: emotion analysis timed out")
	}
	cancel()

	if hint == nil {
		return nil
	}

	if g.emotionHistory != nil {
		g.emotionHistory.Record(sessionID, hint.Emotion, hint.Confidence, hint.Source)
	}
	if g.emotionShift != nil {
		g.emotionShift.Observe(sessionID, string(hint.Emotion), hint.Confidence)
	}

	minConf := 0.5
	if g.personaChain != nil {
		minConf = g.personaChain.FloatFeature(persona.FeatureEmotionMinConfidence, 0.5)
	}
	if hint.Confidence < minConf {
		return nil
	}
	return hint
}

// triggerSelfHeal attempts to generate a plugin for unsupported tasks.
func (g *Gateway) triggerSelfHeal(ctx context.Context, messages []llm.Message, planErr error) {
	if g.healer == nil || len(messages) == 0 {
		return
	}
	lastMsg := messages[len(messages)-1].Content
	errMsg := planErr.Error()
	safego.Go("selfheal", func() {
		healCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if g.healer.ShouldHeal(lastMsg, errMsg) {
			generated, err := g.healer.GenerateAndInstall(healCtx, lastMsg+"\nError: "+errMsg)
			if err != nil {
				slog.Warn("selfheal: generation failed", "err", err)
			} else {
				slog.Info("selfheal: plugin generated and installed", "name", generated.Name)
			}
		}
	})
}

// recordCost tracks token cost for the request.
func (g *Gateway) recordCost(ctx context.Context, req *ChatRequest, tier string, tokensIn, tokensOut int64, start time.Time, span *observe.Span) {
	if g.costTracker == nil {
		return
	}
	model := g.planner.ModelIDForTier(tier)
	cost, alert := g.costTracker.Record(model, req.TenantID, "", req.SessionID, int(tokensIn), int(tokensOut), time.Since(start))
	span.Attrs["cost_usd"] = fmt.Sprintf("%.6f", cost)
	if alert != nil {
		slog.Warn("cost alert", "type", alert.Type, "message", alert.Message)
	}
}

func (g *Gateway) recordRouterOutcome(tierName, modelID string, start time.Time, err error, reply string, span *observe.Span) {
	if g.smartRouter == nil {
		return
	}
	if modelID == "" && g.planner != nil {
		modelID = g.planner.ModelIDForTier(tierName)
	}
	if modelID == "" {
		return
	}

	latency := time.Since(start)
	g.smartRouter.RecordLatency(modelID, latency)

	tier, ok := routerTierFromString(tierName)
	if !ok {
		return
	}
	reward := routerOutcomeReward(err, reply, latency)
	if bandit := g.smartRouter.Bandit(); bandit != nil {
		bandit.RecordOutcome(tier, modelID, reward, float64(latency.Milliseconds()))
	}
	if span != nil {
		span.Attrs["router_outcome_model"] = modelID
		span.Attrs["router_outcome_tier"] = tier.String()
		span.Attrs["router_outcome_reward"] = fmt.Sprintf("%.2f", reward)
		span.Attrs["router_outcome_latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())
	}
}

func routerTierFromString(tierName string) (router.Tier, bool) {
	switch tierName {
	case "fast":
		return router.TierFast, true
	case "smart", "":
		return router.TierSmart, true
	case "expert":
		return router.TierExpert, true
	default:
		return router.TierSmart, false
	}
}

func routerOutcomeReward(err error, reply string, latency time.Duration) float64 {
	if err != nil {
		return 0
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return 0.25
	}

	reward := 0.70
	if len([]rune(reply)) >= 20 {
		reward += 0.15
	} else {
		reward += 0.05
	}

	switch {
	case latency > 60*time.Second:
		reward -= 0.30
	case latency > 30*time.Second:
		reward -= 0.20
	case latency > 10*time.Second:
		reward -= 0.10
	}

	if reward < 0 {
		return 0
	}
	if reward > 1 {
		return 1
	}
	return reward
}

// persistChatResult saves the assistant reply to the session and triggers auto-titling.
func (g *Gateway) persistChatResult(ctx context.Context, req *ChatRequest, result *planner.PlanResult) {
	if req.SessionID == "" {
		return
	}

	assistantContent := result.Reply
	if summary := result.ExecutionSummary(); summary != "" {
		assistantContent = summary + "\n\n" + assistantContent
	}
	var sandboxInfo map[string]any
	if result.Plan != nil {
		sandboxInfo = extractSandboxFromPlan(result.Plan)
	}
	assistantContent = embedSandboxMarker(assistantContent, sandboxInfo)
	g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: assistantContent})

	sess := g.convStore.GetSession(req.SessionID)
	if sess != nil && sess.Name == "" && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
		assistReply := result.Reply
		sessionID := req.SessionID
		safego.Go("auto-title", func() {
			titleCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			title := g.generateConversationTitle(titleCtx, userMsg, assistReply)
			if title != "" {
				g.convStore.Rename(sessionID, title)
			}
		})
	}
}

// runPostChatHooks triggers async post-processing: memory pipeline, learning loop,
// adaptive loop, skill growth, and skill suggestions.
func (g *Gateway) runPostChatHooks(ctx context.Context, req *ChatRequest, result *planner.PlanResult, emotionHint *emotion.Result) {
	if g.orchestrator != nil && result.Reply != "" {
		_ = g.orchestrator.Ingest(ctx, req.TenantID, result.Reply, "conversation", "assistant_reply")
		g.metrics.Cognitive().MemoryIngest.Add(1)
	}

	userMsg := lastUserMessage(req.Messages)

	// Memory pipeline
	if g.pipeline != nil && userMsg != "" {
		reply := result.Reply
		tid := req.TenantID
		safego.Go("memory-pipeline", func() {
			pCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			chatMsgs := []memory.ChatMessage{
				{Role: "user", Content: userMsg},
				{Role: "assistant", Content: reply},
			}
			pResult, err := g.pipeline.Process(pCtx, tid, chatMsgs)
			if err != nil {
				slog.Error("memory pipeline failed", "err", err, "tenant", tid)
			} else if pResult != nil && len(pResult.ExtractedFacts) > 0 {
				if g.factHook != nil {
					g.factHook.OnExtracted(pResult.ExtractedFacts)
				}
				g.ingestFactsToRAG(pCtx, pResult.ExtractedFacts)
			}
		})
	}

	// Learning loop
	if g.learning != nil && userMsg != "" {
		quality := 7
		if g.learning.Reflect() != nil {
			if eval, err := g.learning.Reflect().Evaluate(ctx, userMsg, result.Reply, nil); err == nil {
				quality = eval.Quality
			}
		}
		g.learning.AfterInteraction(ctx, userMsg, result.Reply, result.SkillsUsed, quality)
	}

	// Adaptive loop
	if g.adaptiveLoop != nil && userMsg != "" {
		reply := result.Reply
		safego.Go("adaptive-loop", func() {
			aCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			g.adaptiveLoop.ObserveInteraction(aCtx, userMsg, reply)
		})
		if emotionHint != nil && emotionHint.IsPositive() {
			g.adaptiveLoop.RecordFeedback(adaptive.Feedback{
				Type:        adaptive.FeedbackPreference,
				Dimension:   adaptive.DimEmoji,
				UserMessage: userMsg,
				Correction:  "with_emoji",
			})
		}
	}

	// Skill growth
	if g.skillGrow != nil && userMsg != "" {
		actions := result.SkillsUsed
		safego.Go("skill-grow", func() {
			gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			g.skillGrow.Observe(gCtx, userMsg)
			if len(actions) > 0 {
				g.skillGrow.ObserveActions(gCtx, actions)
			}
		})
	}

	// Skill suggestion
	cnt := g.suggestCounter.Add(1)
	if g.skillSuggester != nil && userMsg != "" && len(result.Reply) > 500 && cnt%5 == 0 {
		reply := result.Reply
		skills := result.SkillsUsed
		sid := req.SessionID
		safego.Go("skill-suggest", func() {
			sCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			suggestions, err := g.skillSuggester.Analyze(sCtx, userMsg, reply, skills)
			if err == nil && len(suggestions.Suggestions) > 0 {
				g.storePendingSuggestions(sid, suggestions.Suggestions)
			}
		})
	}
}

// attachStickerSuggestions adds sticker suggestions to the response.
func (g *Gateway) attachStickerSuggestions(resp *ChatResponse, emotionHint *emotion.Result, platform string) {
	if emotionHint == nil || emotionHint.Emotion == emotion.EmotionNeutral || emotionHint.Emotion == emotion.EmotionUnknown {
		return
	}
	featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
	freq := 2.0
	if g.personaChain != nil {
		freq = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
	}
	if g.stickerMap == nil || !featureOK || mathRandFloat64() >= stickerSendProb(freq) {
		return
	}
	if platform != "" {
		resp.StickerSuggestion = g.stickerMap.Suggest(emotionHint.Emotion, platform)
	} else {
		resp.StickerMulti = g.stickerMap.SuggestMulti(emotionHint.Emotion)
	}
}

func lastUserMessage(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}
