// Package benchmark provides the benchmark runner engine: Scenario interface,
// Runner orchestrator, and ScenarioResult types for the agent-db benchmark harness.
package benchmark

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/repository"
)

// Scenario defines the contract for a single benchmark scenario.
// Each scenario targets one specific database access pattern.
type Scenario interface {
	// Name returns the human-readable name of the scenario.
	Name() string

	// Setup prepares the scenario before warmup and measurement iterations begin.
	// It receives the actual DB-assigned conversation IDs via SeedResult.
	Setup(ctx context.Context, repo repository.ChatRepository, seed SeedResult) error

	// Run executes a single measured iteration of the scenario.
	// Called once per warmup iteration and once per measured iteration.
	Run(ctx context.Context, repo repository.ChatRepository) error

	// Teardown releases any resources held by the scenario.
	// Called once after all iterations (warmup + measured) complete.
	Teardown(ctx context.Context) error
}

// WarmupSkipper is an optional interface. Scenarios that implement it
// with SkipWarmup() returning true will not have warmup iterations run.
// Used by ColdStartLoad (SCEN-04) to measure true cold-start latency.
type WarmupSkipper interface {
	SkipWarmup() bool
}
