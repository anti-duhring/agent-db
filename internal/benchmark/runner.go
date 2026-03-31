package benchmark

import (
	"context"
	"fmt"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/generator"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// RunConfig holds the configuration for a benchmark run (D-08).
type RunConfig struct {
	// Backend is the name of the storage backend being benchmarked (e.g. "postgres").
	Backend string
	// Warmup is the number of un-recorded warm-up iterations per scenario.
	Warmup int
	// Iterations is the number of measured iterations per scenario.
	Iterations int
	// Concurrency is the target concurrent goroutine count (reserved for future use).
	Concurrency int
	// Profile controls the size of the synthetic dataset (Small / Medium / Large).
	Profile generator.Profile
	// Seed is the deterministic seed for data generation.
	Seed int64
}

// SeedResult carries the repository-assigned conversation IDs and the raw
// generated data to scenarios via their Setup call. Because CreateConversation
// generates its own server-side ID, the IDs in OriginalData differ from the
// IDs in Conversations; scenarios must use Conversations for all repo calls.
type SeedResult struct {
	PartnerID     uuid.UUID
	UserID        uuid.UUID
	Conversations []domain.Conversation
	OriginalData  generator.GeneratedData
}

// Runner orchestrates warmup and measured iteration loops across a slice of
// Scenario implementations, recording latency in microseconds to per-scenario
// HdrHistograms (METR-01, METR-02).
type Runner struct {
	repo      repository.ChatRepository
	scenarios []Scenario
	config    RunConfig
}

// NewRunner creates a Runner bound to the given repository, scenarios, and config.
func NewRunner(repo repository.ChatRepository, scenarios []Scenario, config RunConfig) *Runner {
	return &Runner{
		repo:      repo,
		scenarios: scenarios,
		config:    config,
	}
}

// seedRepository populates the repository with the generated data and returns
// the repository-assigned conversations so scenarios can reference real IDs.
func seedRepository(ctx context.Context, repo repository.ChatRepository, data generator.GeneratedData) ([]domain.Conversation, error) {
	seeded := make([]domain.Conversation, 0, len(data.Conversations))

	for _, c := range data.Conversations {
		created, err := repo.CreateConversation(ctx, c.PartnerID, c.UserID)
		if err != nil {
			return nil, fmt.Errorf("seed: create conversation: %w", err)
		}

		// Append messages using the DB-assigned conversation ID.
		for _, msg := range data.Messages[c.ID] {
			_, err := repo.AppendMessage(ctx, created.ID, msg.Role, msg.Content)
			if err != nil {
				return nil, fmt.Errorf("seed: append message: %w", err)
			}
		}

		seeded = append(seeded, created)
	}

	return seeded, nil
}

// Run executes the full benchmark: seed → for each scenario (setup → warmup →
// measured → teardown). Returns a ScenarioResult per scenario.
func (r *Runner) Run(ctx context.Context) ([]ScenarioResult, error) {
	gen := generator.New(r.config.Seed)

	// Fixed partner/user IDs for this run — uniqueness matters, not determinism.
	partnerID := uuid.New()
	userID := uuid.New()

	data := gen.Generate(partnerID, userID, r.config.Profile)

	convs, err := seedRepository(ctx, r.repo, data)
	if err != nil {
		return nil, fmt.Errorf("runner: seed repository: %w", err)
	}

	seedResult := SeedResult{
		PartnerID:     partnerID,
		UserID:        userID,
		Conversations: convs,
		OriginalData:  data,
	}

	results := make([]ScenarioResult, 0, len(r.scenarios))

	for _, sc := range r.scenarios {
		if err := sc.Setup(ctx, r.repo, seedResult); err != nil {
			return nil, fmt.Errorf("runner: scenario %q setup: %w", sc.Name(), err)
		}

		// One histogram per scenario: 1us to 30s, 3 significant digits.
		h := hdrhistogram.New(1, 30_000_000, 3)

		// Warmup pass — iterations are not recorded (METR-02).
		// Scenarios that implement WarmupSkipper and return true skip warmup
		// so that cold-start latency is measured on the first actual read.
		skipWarmup := false
		if ws, ok := sc.(WarmupSkipper); ok {
			skipWarmup = ws.SkipWarmup()
		}
		if !skipWarmup {
			for i := 0; i < r.config.Warmup; i++ {
				if wErr := sc.Run(ctx, r.repo); wErr != nil {
					// Log and continue — warmup failures are tolerated.
					fmt.Printf("runner: warmup iteration %d for %q: %v\n", i, sc.Name(), wErr)
				}
			}
		}

		// Measured pass — latency recorded to histogram in microseconds (METR-01).
		for i := 0; i < r.config.Iterations; i++ {
			start := time.Now()
			if runErr := sc.Run(ctx, r.repo); runErr != nil {
				return nil, fmt.Errorf("runner: scenario %q iteration %d: %w", sc.Name(), i, runErr)
			}
			elapsed := time.Since(start).Microseconds()

			if recErr := h.RecordValue(elapsed); recErr != nil {
				// Out-of-range value — log but don't fail the run.
				fmt.Printf("runner: histogram record out of range for %q iter %d: %v\n", sc.Name(), i, recErr)
			}
		}

		if tdErr := sc.Teardown(ctx); tdErr != nil {
			// Log teardown errors but don't fail the overall run.
			fmt.Printf("runner: scenario %q teardown: %v\n", sc.Name(), tdErr)
		}

		results = append(results, ScenarioResult{
			ScenarioName: sc.Name(),
			P50:          h.ValueAtPercentile(50.0),
			P95:          h.ValueAtPercentile(95.0),
			P99:          h.ValueAtPercentile(99.0),
			TotalCount:   h.TotalCount(),
		})
	}

	return results, nil
}
