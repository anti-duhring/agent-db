package scenarios

import (
	"context"
	"fmt"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time checks: ColdStartScenario must implement benchmark.Scenario
// and benchmark.WarmupSkipper.
var _ benchmark.Scenario = (*ColdStartScenario)(nil)
var _ benchmark.WarmupSkipper = (*ColdStartScenario)(nil)

// ColdStartScenario measures first-read (cold start) latency (SCEN-04).
// It implements WarmupSkipper so the runner skips warm-up iterations,
// ensuring the first Run call hits a cold connection/cache path.
// The actual operation is identical to LoadSlidingWindow — last 20 messages.
type ColdStartScenario struct {
	convID uuid.UUID
}

// NewColdStartScenario returns a new ColdStartScenario ready for Setup.
func NewColdStartScenario() *ColdStartScenario {
	return &ColdStartScenario{}
}

// Name returns the human-readable scenario identifier.
func (s *ColdStartScenario) Name() string {
	return "ColdStartLoad"
}

// SkipWarmup returns true, causing the runner to skip warmup iterations.
func (s *ColdStartScenario) SkipWarmup() bool {
	return true
}

// Setup selects a conversation with 200+ messages when available, otherwise
// falls back to the first conversation. Same selection logic as WindowScenario.
func (s *ColdStartScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return fmt.Errorf("coldstart scenario: no seeded conversations available")
	}

	// Prefer a conversation with 200+ messages for a representative window query.
	for _, conv := range seed.Conversations {
		for _, origConv := range seed.OriginalData.Conversations {
			msgs := seed.OriginalData.Messages[origConv.ID]
			if len(msgs) >= 200 {
				s.convID = conv.ID
				return nil
			}
		}
	}

	// Fallback: use the first conversation regardless of message count.
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run fetches the last 20 messages from the target conversation.
// With warmup skipped, the first iteration measures cold-start read latency.
func (s *ColdStartScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	_, err := repo.LoadWindow(ctx, s.convID, 20)
	return err
}

// Teardown is a no-op for ColdStartScenario.
func (s *ColdStartScenario) Teardown(_ context.Context) error {
	return nil
}
