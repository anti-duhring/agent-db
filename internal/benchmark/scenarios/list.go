package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check.
var _ benchmark.Scenario = (*ListScenario)(nil)

// ListScenario measures list-all-conversations latency for a (partnerID, userID) pair (SCEN-03).
type ListScenario struct {
	partnerID uuid.UUID
	userID    uuid.UUID
}

// NewListScenario creates a new ListScenario.
func NewListScenario() *ListScenario {
	return &ListScenario{}
}

// Name returns the scenario's display name.
func (s *ListScenario) Name() string {
	return "ListConversations"
}

// Setup stores the partner and user IDs from the seed data.
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
