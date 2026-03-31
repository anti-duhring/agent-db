package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check.
var _ benchmark.Scenario = (*ColdStartScenario)(nil)

// ColdStartScenario measures sliding window latency without warmup (SCEN-04).
// It implements WarmupSkipper to signal the runner that warmup should be skipped,
// ensuring all measurements reflect true first-read (cold) latency.
type ColdStartScenario struct {
	convID uuid.UUID
}

// NewColdStartScenario creates a new ColdStartScenario.
func NewColdStartScenario() *ColdStartScenario {
	return &ColdStartScenario{}
}

// Name returns the scenario's display name.
func (s *ColdStartScenario) Name() string {
	return "ColdStartLoad"
}

// SkipWarmup implements benchmark.WarmupSkipper, telling the runner not to
// run warmup iterations before measurement.
func (s *ColdStartScenario) SkipWarmup() bool {
	return true
}

// Setup selects the conversation with the most messages, same logic as WindowScenario.
// NOTE: seed.OriginalData.Messages is keyed by generator-assigned conversation IDs,
// while seed.Conversations holds DB-assigned conversations. We match by index order.
func (s *ColdStartScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return nil
	}

	// Prefer a conversation with 200+ messages.
	// Match by index: seed.Conversations[i] corresponds to OriginalData.Conversations[i].
	for i, origConv := range seed.OriginalData.Conversations {
		if i >= len(seed.Conversations) {
			break
		}
		if msgs, ok := seed.OriginalData.Messages[origConv.ID]; ok && len(msgs) >= 200 {
			s.convID = seed.Conversations[i].ID
			return nil
		}
	}

	// Fallback: use the first conversation.
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run fetches the last 20 messages without prior warmup.
func (s *ColdStartScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	_, err := repo.LoadWindow(ctx, s.convID, 20)
	return err
}

// Teardown is a no-op for ColdStartScenario.
func (s *ColdStartScenario) Teardown(_ context.Context) error {
	return nil
}
