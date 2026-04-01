// Package scenarios provides concrete implementations of the benchmark.Scenario
// interface, each targeting a distinct database access pattern.
package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check.
var _ benchmark.Scenario = (*AppendScenario)(nil)

// AppendScenario measures single-message write latency (SCEN-01).
type AppendScenario struct {
	convID   uuid.UUID
	msgIndex int
}

// NewAppendScenario creates a new AppendScenario.
func NewAppendScenario() *AppendScenario {
	return &AppendScenario{}
}

// Name returns the scenario's display name.
func (s *AppendScenario) Name() string {
	return "AppendMessage"
}

// Setup stores the first conversation ID from the seeded data.
func (s *AppendScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return nil
	}
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run appends a single message and measures its write latency.
func (s *AppendScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	s.msgIndex++
	_, err := repo.AppendMessage(ctx, s.convID, domain.RoleUser, "benchmark message")
	return err
}

// Teardown is a no-op for AppendScenario.
func (s *AppendScenario) Teardown(_ context.Context) error {
	return nil
}
