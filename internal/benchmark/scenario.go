// Package benchmark provides the runner engine for executing benchmark scenarios
// against chat storage backends. It defines the Scenario interface, the Runner
// orchestrator, and result types with HdrHistogram-based latency measurement.
package benchmark

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/repository"
)

// WarmupSkipper is an optional interface. Scenarios that implement it
// with SkipWarmup() returning true will not have warmup iterations run by
// the Runner. Used by ColdStartLoad (SCEN-04) to measure first-read latency.
type WarmupSkipper interface {
	SkipWarmup() bool
}

// Scenario is the interface that all benchmark scenarios must implement.
// Each scenario exercises a specific chat storage operation or sequence of
// operations against a ChatRepository backend (D-06).
type Scenario interface {
	// Name returns a short human-readable identifier for this scenario.
	// Used as the row label in the results table.
	Name() string

	// Setup is called once before the warmup and measured iterations begin.
	// It receives the pre-seeded data so scenarios can look up conversation IDs,
	// user IDs, and other identifiers they need to issue operations against.
	Setup(ctx context.Context, repo repository.ChatRepository, seed SeedResult) error

	// Run executes the operation being benchmarked. It is called once per
	// iteration (both warmup and measured). Latency is measured around this call.
	Run(ctx context.Context, repo repository.ChatRepository) error

	// Teardown is called once after all iterations complete. It may be used to
	// clean up any state created during Setup or Run. Errors are logged but do
	// not fail the run.
	Teardown(ctx context.Context) error
}
