package scenarios

import (
	"context"
	"fmt"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time check: WindowScenario must implement benchmark.Scenario.
var _ benchmark.Scenario = (*WindowScenario)(nil)

// WindowScenario measures sliding window read latency (SCEN-02).
// Each Run call fetches the last 20 messages from the target conversation.
// The target is preferably a conversation with 200+ messages; if the profile
// is too small, the first available conversation is used instead.
type WindowScenario struct {
	convID uuid.UUID
}

// NewWindowScenario returns a new WindowScenario ready for Setup.
func NewWindowScenario() *WindowScenario {
	return &WindowScenario{}
}

// Name returns the human-readable scenario identifier.
func (s *WindowScenario) Name() string {
	return "LoadSlidingWindow"
}

// Setup selects a conversation with 200+ messages when available, otherwise falls
// back to the first conversation. Stores the conversation ID for Run.
func (s *WindowScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return fmt.Errorf("window scenario: no seeded conversations available")
	}

	// Prefer a conversation whose generated message list has 200+ entries so the
	// window query is representative. If none qualifies (e.g. small profile with
	// only 10 messages), fall back to the first conversation.
	for _, conv := range seed.Conversations {
		// seed.OriginalData.Messages is keyed by generated UUID, not the repo-assigned ID.
		// Iterate the original conversations to find the one that maps to this repo conv.
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
func (s *WindowScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	_, err := repo.LoadWindow(ctx, s.convID, 20)
	return err
}

// Teardown is a no-op for WindowScenario.
func (s *WindowScenario) Teardown(_ context.Context) error {
	return nil
}
