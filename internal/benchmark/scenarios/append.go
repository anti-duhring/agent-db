// Package scenarios provides concrete Scenario implementations for the benchmark harness.
// Each scenario targets a specific ChatRepository access pattern.
package scenarios

import (
	"context"
	"fmt"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time check: AppendScenario must implement benchmark.Scenario.
var _ benchmark.Scenario = (*AppendScenario)(nil)

// AppendScenario measures single-message write latency (SCEN-01).
// Each Run call appends one message to the target conversation.
type AppendScenario struct {
	convID   uuid.UUID
	msgIndex int
}

// NewAppendScenario returns a new AppendScenario ready for Setup.
func NewAppendScenario() *AppendScenario {
	return &AppendScenario{}
}

// Name returns the human-readable scenario identifier.
func (s *AppendScenario) Name() string {
	return "AppendMessage"
}

// Setup stores the first seeded conversation ID for use during Run.
func (s *AppendScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return fmt.Errorf("append scenario: no seeded conversations available")
	}
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run appends a single message to the target conversation and measures write latency.
func (s *AppendScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	s.msgIndex++
	_, err := repo.AppendMessage(ctx, s.convID, domain.RoleUser, "benchmark message")
	return err
}

// Teardown is a no-op for AppendScenario.
func (s *AppendScenario) Teardown(_ context.Context) error {
	return nil
}
