package cognikernel

import (
	"context"
	"time"

	"yunque-agent/internal/agentcore/localbrain"
)

// LocalBrainDistillAdapter bridges the CogniKernel's SelfDistillSink
// interface to the localbrain package's SelfDistillSink (IntentTrainingSample).
//
// CogniKernel produces TrainingSample from the reflective loop;
// this adapter converts it to IntentTrainingSample and feeds it
// into the localbrain's training pipeline.
type LocalBrainDistillAdapter struct {
	sink localbrain.SelfDistillSink
}

// NewLocalBrainDistillAdapter wraps a localbrain.SelfDistillSink.
func NewLocalBrainDistillAdapter(sink localbrain.SelfDistillSink) *LocalBrainDistillAdapter {
	return &LocalBrainDistillAdapter{sink: sink}
}

// PushSample converts a CogniKernel TrainingSample to a localbrain
// IntentTrainingSample and ingests it.
func (a *LocalBrainDistillAdapter) PushSample(_ context.Context, sample TrainingSample) error {
	if a.sink == nil {
		return nil
	}

	lbSample := localbrain.IntentTrainingSample{
		UserQuery: sample.Input,
		TenantID:  sample.TenantID,
		RouteIntent: localbrain.Intent{
			NeedTools: len(sample.SkillsUsed) > 0,
		},
		RouteTier:      sample.ModelTier,
		RouteSatisfied: sample.Score >= 7.0,
		Score:          sample.Score,
		Source:         "cognikernel_reflect",
		Timestamp:      time.Now(),
	}

	return a.sink.IngestConversation(lbSample)
}
