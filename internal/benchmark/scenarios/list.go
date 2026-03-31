package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time check: ListScenario must implement benchmark.Scenario.
var _ benchmark.Scenario = (*ListScenario)(nil)

// ListScenario measures conversation-list latency (SCEN-03).
// Each Run call fetches all conversations for the seeded (partner_id, user_id) pair.
type ListScenario struct {
	partnerID uuid.UUID
	userID    uuid.UUID
}

// NewListScenario returns a new ListScenario ready for Setup.
func NewListScenario() *ListScenario {
	return &ListScenario{}
}

// Name returns the human-readable scenario identifier.
func (s *ListScenario) Name() string {
	return "ListConversations"
}

// Setup stores the partner and user IDs from the seed for use during Run.
func (s *ListScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	s.partnerID = seed.PartnerID
	s.userID = seed.UserID
	return nil
}

// Run lists all conversations for the seeded (partnerID, userID) pair.
func (s *ListScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	_, err := repo.ListConversations(ctx, s.partnerID, s.userID)
	return err
}

// Teardown is a no-op for ListScenario.
func (s *ListScenario) Teardown(_ context.Context) error {
	return nil
}
