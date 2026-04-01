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

// RunConfig holds the configuration for a benchmark run.
type RunConfig struct {
	Backend     string
	Warmup      int
	Iterations  int
	Concurrency int
	Profile     generator.Profile
	Seed        int64
}

// Runner orchestrates scenario execution, separating warmup from measurement,
// and produces percentile latency data via HdrHistogram.
type Runner struct {
	repo      repository.ChatRepository
	scenarios []Scenario
	config    RunConfig
}

// NewRunner creates a new Runner with the given repository, scenarios, and config.
func NewRunner(repo repository.ChatRepository, scenarios []Scenario, config RunConfig) *Runner {
	return &Runner{
		repo:      repo,
		scenarios: scenarios,
		config:    config,
	}
}

// SeedResult carries the DB-assigned conversation IDs after seeding, so
// scenarios can use the actual (FK-safe) conversation IDs in their runs.
type SeedResult struct {
	PartnerID     uuid.UUID
	UserID        uuid.UUID
	Conversations []domain.Conversation
	OriginalData  generator.GeneratedData
}

// seedRepository inserts all generated conversations and messages into the
// repository, returning the actual DB-assigned conversation structs.
// The generator creates conversation IDs that differ from DB-assigned IDs
// because CreateConversation generates its own ID. We map generated messages
// to the correct DB conversation ID using the original order.
func seedRepository(ctx context.Context, repo repository.ChatRepository, data generator.GeneratedData) ([]domain.Conversation, error) {
	created := make([]domain.Conversation, 0, len(data.Conversations))

	for _, genConv := range data.Conversations {
		dbConv, err := repo.CreateConversation(ctx, genConv.PartnerID, genConv.UserID)
		if err != nil {
			return nil, fmt.Errorf("seedRepository: create conversation: %w", err)
		}

		// Append messages using the DB-assigned conversation ID.
		for _, msg := range data.Messages[genConv.ID] {
			_, err := repo.AppendMessage(ctx, dbConv.ID, msg.Role, msg.Content)
			if err != nil {
				return nil, fmt.Errorf("seedRepository: append message: %w", err)
			}
		}

		created = append(created, dbConv)
	}

	return created, nil
}

// Run executes all registered scenarios and returns their latency results.
// For each scenario:
//   - Setup is called once with seeded repository data
//   - Warmup iterations are run without recording (unless WarmupSkipper)
//   - Measured iterations are timed and recorded to HdrHistogram
//   - Teardown is called once after all iterations
func (r *Runner) Run(ctx context.Context) ([]ScenarioResult, error) {
	gen := generator.New(r.config.Seed)

	// Use fixed UUIDs derived from seed for deterministic partner/user scoping.
	partnerID := uuid.New()
	userID := uuid.New()

	data := gen.Generate(partnerID, userID, r.config.Profile)

	fmt.Printf("Seeding repository with %d conversations...\n", len(data.Conversations))
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
		fmt.Printf("Running scenario: %s\n", sc.Name())

		if err := sc.Setup(ctx, r.repo, seedResult); err != nil {
			return nil, fmt.Errorf("runner: scenario %s setup: %w", sc.Name(), err)
		}

		// HdrHistogram: 1 microsecond to 30 seconds, 3 significant digits.
		h := hdrhistogram.New(1, 30_000_000, 3)

		// Warmup phase — skip if scenario implements WarmupSkipper.
		skipWarmup := false
		if ws, ok := sc.(WarmupSkipper); ok {
			skipWarmup = ws.SkipWarmup()
		}
		if !skipWarmup {
			for i := 0; i < r.config.Warmup; i++ {
				_ = sc.Run(ctx, r.repo)
			}
		}

		// Measured phase.
		for i := 0; i < r.config.Iterations; i++ {
			start := time.Now()
			if err := sc.Run(ctx, r.repo); err != nil {
				return nil, fmt.Errorf("runner: scenario %s iteration %d: %w", sc.Name(), i, err)
			}
			elapsed := time.Since(start).Microseconds()
			if recErr := h.RecordValue(elapsed); recErr != nil {
				fmt.Printf("warning: histogram out-of-range value %d for scenario %s: %v\n", elapsed, sc.Name(), recErr)
			}
		}

		if err := sc.Teardown(ctx); err != nil {
			return nil, fmt.Errorf("runner: scenario %s teardown: %w", sc.Name(), err)
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
