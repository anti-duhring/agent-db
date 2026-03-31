package scenarios

import (
	"context"
	"fmt"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// Compile-time check: ConcurrentScenario must implement benchmark.Scenario.
var _ benchmark.Scenario = (*ConcurrentScenario)(nil)

// ConcurrentScenario measures throughput under parallel write pressure (SCEN-05).
// Each Run call spawns concurrency goroutines that simultaneously call AppendMessage.
// The runner's outer loop handles per-call timing; each Run iteration represents
// one round of N concurrent writes completing.
//
// The scenario also keeps a per-goroutine histogram for internal analysis,
// though the runner's external histogram captures total-round latency.
type ConcurrentScenario struct {
	convID      uuid.UUID
	concurrency int
	histogram   *hdrhistogram.Histogram
}

// NewConcurrentScenario returns a ConcurrentScenario configured to spawn
// concurrency goroutines per Run call.
func NewConcurrentScenario(concurrency int) *ConcurrentScenario {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &ConcurrentScenario{
		concurrency: concurrency,
		// 1us to 30s, 3 significant digits — matches runner histogram settings.
		histogram: hdrhistogram.New(1, 30_000_000, 3),
	}
}

// Name returns the human-readable scenario identifier.
func (s *ConcurrentScenario) Name() string {
	return "ConcurrentWrites"
}

// Setup stores the first seeded conversation ID for use during Run.
func (s *ConcurrentScenario) Setup(_ context.Context, _ repository.ChatRepository, seed benchmark.SeedResult) error {
	if len(seed.Conversations) == 0 {
		return fmt.Errorf("concurrent scenario: no seeded conversations available")
	}
	s.convID = seed.Conversations[0].ID
	return nil
}

// Run spawns s.concurrency goroutines that each call AppendMessage once,
// using errgroup for lifecycle management. Returns the first error encountered.
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
