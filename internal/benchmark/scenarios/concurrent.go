package scenarios

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// Compile-time interface check.
var _ benchmark.Scenario = (*ConcurrentScenario)(nil)

// ConcurrentScenario measures concurrent write throughput by spawning N goroutines
// that each append a message in parallel (SCEN-05). Each Run() call is one
// "iteration" that issues N concurrent writes; the runner's histogram records
// the total wall-clock time for all N writes to complete.
type ConcurrentScenario struct {
	convID      uuid.UUID
	concurrency int
}

// NewConcurrentScenario creates a new ConcurrentScenario with the given goroutine count.
func NewConcurrentScenario(concurrency int) *ConcurrentScenario {
	return &ConcurrentScenario{concurrency: concurrency}
}

// Name returns the scenario's display name.
func (s *ConcurrentScenario) Name() string {
	return "ConcurrentWrites"
}

// Setup stores the first conversation ID for concurrent append targets.
func (s *ConcurrentScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return nil
	}
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run spawns s.concurrency goroutines, each appending one message, and waits
// for all to complete. Returns the first error if any goroutine fails.
func (s *ConcurrentScenario) Run(ctx context.Context, repo repository.ChatRepository) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for i := 0; i < s.concurrency; i++ {
		g.Go(func() error {
			_, err := repo.AppendMessage(gctx, s.convID, domain.RoleUser, "concurrent benchmark message")
			return err
		})
	}

	return g.Wait()
}

// Teardown is a no-op for ConcurrentScenario.
func (s *ConcurrentScenario) Teardown(_ context.Context) error {
	return nil
}
