package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/anti-duhring/agent-db/internal/benchmark"
	"github.com/anti-duhring/agent-db/internal/benchmark/scenarios"
	"github.com/anti-duhring/agent-db/internal/generator"
	"github.com/anti-duhring/agent-db/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func main() {
	backend := flag.String("backend", "postgres", "backend to benchmark (postgres)")
	scenario := flag.String("scenario", "all", "scenario(s) to run (all,append,window,list,coldstart,concurrent)")
	profile := flag.String("profile", "medium", "data profile (small,medium,large)")
	iters := flag.Int("iterations", 100, "measured iteration count per scenario")
	warmup := flag.Int("warmup", 10, "warmup iteration count (discarded)")
	conc := flag.Int("concurrency", 10, "goroutine count for concurrent scenario")
	seed := flag.Int64("seed", 42, "RNG seed for deterministic data")
	dryRun := flag.Bool("dry-run", false, "verify connectivity without running benchmarks")
	flag.Parse()

	// Validate profile flag.
	var prof generator.Profile
	switch *profile {
	case "small":
		prof = generator.Small
	case "medium":
		prof = generator.Medium
	case "large":
		prof = generator.Large
	default:
		fmt.Fprintf(os.Stderr, "unknown profile: %s (valid: small, medium, large)\n", *profile)
		os.Exit(1)
	}

	// Validate backend flag — only "postgres" is valid in Phase 2.
	if *backend != "postgres" {
		fmt.Fprintf(os.Stderr, "unknown backend: %s (valid: postgres)\n", *backend)
		os.Exit(1)
	}

	ctx := context.Background()

	// Start Postgres testcontainer.
	fmt.Println("Starting Postgres container...")
	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("agentdb"),
		tcpostgres.WithUsername("bench"),
		tcpostgres.WithPassword("bench"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer testcontainers.TerminateContainer(ctr)

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	repo, err := postgres.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create postgres repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// --dry-run: verify connectivity and schema without running benchmarks.
	if *dryRun {
		fmt.Println("Dry run - verifying setup:")
		fmt.Println("  [PASS] Postgres container started")
		fmt.Println("  [PASS] Connection established")
		fmt.Println("  [PASS] Schema applied")

		// Verify seed data insertion.
		conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
		if err != nil {
			fmt.Printf("  [FAIL] Seed data: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  [PASS] Seed data insertion")

		// Sample query.
		_, err = repo.LoadWindow(ctx, conv.ID, 1)
		if err != nil {
			fmt.Printf("  [FAIL] Sample query: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  [PASS] Sample query")
		fmt.Println("\nDry run complete - all checks passed")
		os.Exit(0)
	}

	// Build scenario list from --scenario flag.
	scenarioMap := map[string]benchmark.Scenario{
		"append":     scenarios.NewAppendScenario(),
		"window":     scenarios.NewWindowScenario(),
		"list":       scenarios.NewListScenario(),
		"coldstart":  scenarios.NewColdStartScenario(),
		"concurrent": scenarios.NewConcurrentScenario(*conc),
	}
	allNames := []string{"append", "window", "list", "coldstart", "concurrent"}

	var selectedScenarios []benchmark.Scenario
	if *scenario == "all" {
		for _, name := range allNames {
			selectedScenarios = append(selectedScenarios, scenarioMap[name])
		}
	} else {
		for _, name := range strings.Split(*scenario, ",") {
			name = strings.TrimSpace(name)
			sc, ok := scenarioMap[name]
			if !ok {
				fmt.Fprintf(os.Stderr, "unknown scenario: %s (valid: all,append,window,list,coldstart,concurrent)\n", name)
				os.Exit(1)
			}
			selectedScenarios = append(selectedScenarios, sc)
		}
	}

	// Create RunConfig and Runner, then execute benchmarks.
	config := benchmark.RunConfig{
		Backend:     *backend,
		Warmup:      *warmup,
		Iterations:  *iters,
		Concurrency: *conc,
		Profile:     prof,
		Seed:        *seed,
	}

	runner := benchmark.NewRunner(repo, selectedScenarios, config)
	results, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}

	benchmark.PrintResults(*backend, prof.Name, *iters, *seed, results)
}
