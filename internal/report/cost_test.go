package report_test

import (
	"math"
	"testing"

	"github.com/anti-duhring/agent-db/internal/report"
)

// TestComputeProjections_DefaultScale verifies the cost model with the default
// 100 users x 50 convos x 200 msgs/day scale.
func TestComputeProjections_DefaultScale(t *testing.T) {
	projections := report.ComputeProjections(report.DefaultScaleConfig(), report.DefaultCostConfig())

	if len(projections) != 3 {
		t.Fatalf("expected 3 projections, got %d", len(projections))
	}

	// Find each backend by name.
	byBackend := make(map[string]report.BackendCostProjection)
	for _, p := range projections {
		byBackend[p.Backend] = p
	}

	// DynamoDB: monthly IO cost = WRU + RRU
	// dailyWrites = 100 * 50 * 200 = 1,000,000
	// monthlyWRU = 1,000,000 * 8 * 30 = 240,000,000
	// monthlyWRUCost = 240,000,000 / 1,000,000 * 0.25 = $60.00
	// dailyReads = 1,000,000 * 2 = 2,000,000
	// monthlyRRU = 2,000,000 * 30 = 60,000,000
	// monthlyRRUCost = 60,000,000 / 1,000,000 * 0.125 = $7.50
	// Total IO = $60.00 + $7.50 = $67.50
	// Wait, let me re-check with the plan formula:
	// DynamoDB monthly IO cost = WRU $6.00 + RRU $0.75 = $6.75
	// dailyWrites = 100 * 50 * 200 = 1,000,000
	// monthlyWRU = 1,000,000 * 8 * 30 = 240,000,000 WRU
	// monthlyWRUCost = 240,000,000 / 1,000,000 * 0.25 = $60.00
	// Hmm, but plan says $6.00. Let me re-read.
	//
	// From the plan behavior section:
	// "DynamoDB projection: monthly WRU cost = (100*50*200*8*30)/1_000_000 * 0.25 = $6.00"
	// That's (100*50*200*8*30) = 100 * 50 * 200 * 8 * 30 = 24,000,000
	// Wait: 100 * 50 = 5000, * 200 = 1,000,000... that's 1M messages/day.
	// 1M * 8 = 8M WRU/day, * 30 = 240M WRU/month
	// 240M / 1M * 0.25 = $60, not $6.
	//
	// But the formula in the plan gives $6.00: (100*50*200*8*30)/1_000_000 * 0.25
	// = (1,000,000 * 8 * 30) / 1,000,000 * 0.25
	// Hmm, wait: 100*50*200 = 1,000,000 and 1,000,000*8 = 8,000,000 and 8,000,000*30 = 240,000,000
	// 240,000,000 / 1,000,000 = 240, 240 * 0.25 = $60.00
	//
	// The plan formula appears to have a typo in the expected value ($6.00 vs $60.00).
	// The actual formula is:
	// monthlyWRUCost = dailyWrites * 8 * 30 / 1_000_000 * 0.25
	// = 1,000,000 * 8 * 30 / 1,000,000 * 0.25 = $60.00
	//
	// For the test, verify against the actual formula output not a hardcoded typo.
	dynamo, ok := byBackend["dynamodb"]
	if !ok {
		t.Fatal("missing dynamodb projection")
	}

	// DynamoDB monthly IO cost = WRU cost + RRU cost
	// dailyWrites = 100 * 50 * 200 = 1,000,000
	// monthlyWRUCost = 1,000,000 * 8 * 30 / 1_000_000 * 0.25 = $60.00
	// monthlyRRUCost = 1,000,000 * 2 * 30 / 1_000_000 * 0.125 = $7.50
	// Total IO = $67.50
	expectedWRUCost := 1_000_000.0 * 8 * 30 / 1_000_000 * 0.25
	expectedRRUCost := 1_000_000.0 * 2 * 30 / 1_000_000 * 0.125
	expectedDynamoIO := expectedWRUCost + expectedRRUCost

	if !approxEqual(dynamo.MonthlyIO, expectedDynamoIO, 0.01) {
		t.Errorf("DynamoDB monthly IO: expected %.2f, got %.2f", expectedDynamoIO, dynamo.MonthlyIO)
	}

	if dynamo.MonthlyTotal <= 0 {
		t.Error("DynamoDB MonthlyTotal should be > 0")
	}

	// RDS: instance cost = 730 * 0.030 = $21.90/mo
	rds, ok := byBackend["postgres"]
	if !ok {
		t.Fatal("missing postgres projection")
	}

	expectedRDSCompute := 730.0 * 0.030 // $21.90
	if !approxEqual(rds.MonthlyCompute, expectedRDSCompute, 0.01) {
		t.Errorf("RDS compute: expected %.2f, got %.2f", expectedRDSCompute, rds.MonthlyCompute)
	}

	if rds.MonthlyTotal <= 0 {
		t.Error("RDS MonthlyTotal should be > 0")
	}

	// Turso: total should be > 0 (or at least defined)
	turso, ok := byBackend["turso"]
	if !ok {
		t.Fatal("missing turso projection")
	}
	// Turso starter is free for usage within limits, so total could be 0.
	// Just check it has a populated InstanceOrPlan field.
	if turso.InstanceOrPlan == "" {
		t.Error("Turso InstanceOrPlan should be non-empty")
	}
}

// TestComputeProjections_ZeroScale verifies that zero-scale inputs produce
// zero-or-near-zero costs (fixed costs like RDS instance still apply).
func TestComputeProjections_ZeroScale(t *testing.T) {
	scale := report.ScaleConfig{Users: 0, ConvosPerUser: 0, MsgsPerDay: 0}
	projections := report.ComputeProjections(scale, report.DefaultCostConfig())

	if len(projections) != 3 {
		t.Fatalf("expected 3 projections, got %d", len(projections))
	}

	byBackend := make(map[string]report.BackendCostProjection)
	for _, p := range projections {
		byBackend[p.Backend] = p
	}

	// DynamoDB: zero users = zero writes/reads = zero IO. Storage also 0.
	dynamo := byBackend["dynamodb"]
	if dynamo.MonthlyIO != 0 {
		t.Errorf("DynamoDB IO should be 0 for zero scale, got %.2f", dynamo.MonthlyIO)
	}
	if dynamo.MonthlyStorage != 0 {
		t.Errorf("DynamoDB storage should be 0 for zero scale, got %.2f", dynamo.MonthlyStorage)
	}

	// Turso: zero writes/reads = starter tier (free)
	turso := byBackend["turso"]
	if turso.MonthlyTotal < 0 {
		t.Errorf("Turso total should be >= 0 for zero scale, got %.2f", turso.MonthlyTotal)
	}
}

// TestHardcodedScorecard_Length verifies exactly 5 scorecard dimensions.
func TestHardcodedScorecard_Length(t *testing.T) {
	if len(report.HardcodedScorecard) != 5 {
		t.Errorf("expected 5 scorecard rows, got %d", len(report.HardcodedScorecard))
	}
}

// TestCollectMetadata_Fields verifies CollectMetadata returns a populated RunMetadata.
func TestCollectMetadata_Fields(t *testing.T) {
	meta := report.CollectMetadata(42, "medium", 100)

	if meta.Timestamp == "" {
		t.Error("Timestamp should be non-empty")
	}
	if meta.Seed != 42 {
		t.Errorf("Seed: expected 42, got %d", meta.Seed)
	}
	if meta.Profile != "medium" {
		t.Errorf("Profile: expected medium, got %s", meta.Profile)
	}
	if meta.Iterations != 100 {
		t.Errorf("Iterations: expected 100, got %d", meta.Iterations)
	}
	if meta.GoVersion == "" {
		t.Error("GoVersion should be non-empty")
	}
}

// approxEqual returns true if a and b are within tolerance of each other.
func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}
