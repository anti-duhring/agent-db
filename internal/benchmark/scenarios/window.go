package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check.
var _ benchmark.Scenario = (*WindowScenario)(nil)

// WindowScenario measures sliding window read latency (SCEN-02).
// It fetches the last 20 messages from a conversation, preferring a
// conversation with 200+ messages for a realistic workload.
type WindowScenario struct {
	convID uuid.UUID
}

// NewWindowScenario creates a new WindowScenario.
func NewWindowScenario() *WindowScenario {
	return &WindowScenario{}
}

// Name returns the scenario's display name.
func (s *WindowScenario) Name() string {
	return "LoadSlidingWindow"
}

// Setup selects the conversation with the most messages for the window read.
// Falls back to the first conversation if no large conversation is found.
// NOTE: seed.OriginalData.Messages is keyed by generator-assigned conversation IDs,
// while seed.Conversations holds DB-assigned conversations. We match by index order.
func (s *WindowScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return nil
	}

	// Prefer a conversation with 200+ messages for a realistic window read.
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

	// Fallback: use the first conversation regardless of message count
	// (handles the "small" profile with 10 messages).
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run fetches the last 20 messages from the selected conversation.
func (s *WindowScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	_, err := repo.LoadWindow(ctx, s.convID, 20)
	return err
}

// Teardown is a no-op for WindowScenario.
func (s *WindowScenario) Teardown(_ context.Context) error {
	return nil
}
