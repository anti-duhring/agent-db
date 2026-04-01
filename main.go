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
	dynamodbrepo "github.com/anti-duhring/agent-db/internal/repository/dynamodb"
	"github.com/anti-duhring/agent-db/internal/repository/postgres"
	tursorepo "github.com/anti-duhring/agent-db/internal/repository/turso"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

func main() {
	backend := flag.String("backend", "postgres", "backend(s) to benchmark (postgres,dynamodb,turso,all or comma-separated)")
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

	// Parse --backend flag into a list of backends to run.
	validBackends := map[string]bool{"postgres": true, "dynamodb": true, "turso": true}
	allBackendNames := []string{"postgres", "dynamodb", "turso"}

	var selectedBackends []string
	backendArg := *backend
	isAll := backendArg == "all"
	if isAll {
		selectedBackends = allBackendNames
	} else {
		for _, b := range strings.Split(backendArg, ",") {
			b = strings.TrimSpace(b)
			if !validBackends[b] {
				fmt.Fprintf(os.Stderr, "unknown backend: %s (valid: postgres, dynamodb, turso, all)\n", b)
				os.Exit(1)
			}
			selectedBackends = append(selectedBackends, b)
		}
	}

	// Build scenario list from --scenario flag.
	scenarioFactory := func(concurrency int) map[string]benchmark.Scenario {
		return map[string]benchmark.Scenario{
			"append":     scenarios.NewAppendScenario(),
			"window":     scenarios.NewWindowScenario(),
			"list":       scenarios.NewListScenario(),
			"coldstart":  scenarios.NewColdStartScenario(),
			"concurrent": scenarios.NewConcurrentScenario(concurrency),
		}
	}
	allScenarioNames := []string{"append", "window", "list", "coldstart", "concurrent"}

	buildScenarios := func() []benchmark.Scenario {
		scenarioMap := scenarioFactory(*conc)
		var selected []benchmark.Scenario
		if *scenario == "all" {
			for _, name := range allScenarioNames {
				selected = append(selected, scenarioMap[name])
			}
		} else {
			for _, name := range strings.Split(*scenario, ",") {
				name = strings.TrimSpace(name)
				sc, ok := scenarioMap[name]
				if !ok {
					fmt.Fprintf(os.Stderr, "unknown scenario: %s (valid: all,append,window,list,coldstart,concurrent)\n", name)
					os.Exit(1)
				}
				selected = append(selected, sc)
			}
		}
		return selected
	}

	ctx := context.Background()

	for _, b := range selectedBackends {
		switch b {
		case "postgres":
			if *dryRun {
				runPostgresDryRun(ctx)
			} else {
				runPostgres(ctx, prof, buildScenarios(), *iters, *warmup, *conc, *seed)
			}
		case "dynamodb":
			if *dryRun {
				runDynamoDBDryRun(ctx)
			} else {
				runDynamoDB(ctx, prof, buildScenarios(), *iters, *warmup, *conc, *seed)
			}
		case "turso":
			if *dryRun {
				runTursoDryRun(ctx, isAll)
			} else {
				runTurso(ctx, prof, buildScenarios(), *iters, *warmup, *conc, *seed, isAll)
			}
		}
	}
}

// runPostgres starts a Postgres testcontainer, creates the repository, runs all scenarios,
// and prints results with BackendMeta.
func runPostgres(ctx context.Context, prof generator.Profile, selectedScenarios []benchmark.Scenario, iters, warmup, conc int, seed int64) {
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
		fmt.Fprintf(os.Stderr, "failed to get postgres connection string: %v\n", err)
		os.Exit(1)
	}

	repo, err := postgres.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create postgres repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	config := benchmark.RunConfig{
		Backend:     "postgres",
		Warmup:      warmup,
		Iterations:  iters,
		Concurrency: conc,
		Profile:     prof,
		Seed:        seed,
	}

	runner := benchmark.NewRunner(repo, selectedScenarios, config)
	results, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres benchmark failed: %v\n", err)
		os.Exit(1)
	}

	meta := benchmark.BackendMeta{
		Name:      "postgres",
		Transport: "pgx/v5 (local container)",
	}
	benchmark.PrintResults(meta, prof.Name, iters, seed, results)
}

// runPostgresDryRun verifies Postgres connectivity and schema without running benchmarks.
func runPostgresDryRun(ctx context.Context) {
	fmt.Println("=== Postgres dry-run ===")
	fmt.Println("Starting Postgres container...")
	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("agentdb"),
		tcpostgres.WithUsername("bench"),
		tcpostgres.WithPassword("bench"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] Postgres container: %v\n", err)
		os.Exit(1)
	}
	defer testcontainers.TerminateContainer(ctr)

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] Connection string: %v\n", err)
		os.Exit(1)
	}

	repo, err := postgres.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] Repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	fmt.Println("  [PASS] Postgres container started")
	fmt.Println("  [PASS] Connection established")
	fmt.Println("  [PASS] Schema applied")

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		fmt.Printf("  [FAIL] Seed data: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Seed data insertion")

	_, err = repo.LoadWindow(ctx, conv.ID, 1)
	if err != nil {
		fmt.Printf("  [FAIL] Sample query: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Sample query")
	fmt.Println("  Postgres dry run complete")
}

// runDynamoDB starts a LocalStack container, creates the DynamoDB repository, runs all scenarios,
// and prints results with BackendMeta.
func runDynamoDB(ctx context.Context, prof generator.Profile, selectedScenarios []benchmark.Scenario, iters, warmup, conc int, seed int64) {
	fmt.Println("Starting LocalStack container for DynamoDB...")
	lsCtr, err := localstack.Run(ctx, "localstack/localstack:3.8")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start LocalStack container: %v\n", err)
		os.Exit(1)
	}
	defer testcontainers.TerminateContainer(lsCtr)

	host, err := lsCtr.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get LocalStack host: %v\n", err)
		os.Exit(1)
	}
	port, err := lsCtr.MappedPort(ctx, "4566/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get LocalStack port: %v\n", err)
		os.Exit(1)
	}
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	repo, err := dynamodbrepo.New(ctx, endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create DynamoDB repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	config := benchmark.RunConfig{
		Backend:     "dynamodb",
		Warmup:      warmup,
		Iterations:  iters,
		Concurrency: conc,
		Profile:     prof,
		Seed:        seed,
	}

	runner := benchmark.NewRunner(repo, selectedScenarios, config)
	results, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dynamodb benchmark failed: %v\n", err)
		os.Exit(1)
	}

	meta := benchmark.BackendMeta{
		Name:      "dynamodb",
		Transport: "aws-sdk-go-v2 (LocalStack)",
	}
	benchmark.PrintResults(meta, prof.Name, iters, seed, results)
}

// runDynamoDBDryRun verifies DynamoDB (LocalStack) connectivity without running benchmarks.
func runDynamoDBDryRun(ctx context.Context) {
	fmt.Println("=== DynamoDB dry-run ===")
	fmt.Println("Starting LocalStack container...")
	lsCtr, err := localstack.Run(ctx, "localstack/localstack:3.8")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] LocalStack container: %v\n", err)
		os.Exit(1)
	}
	defer testcontainers.TerminateContainer(lsCtr)

	host, err := lsCtr.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] LocalStack host: %v\n", err)
		os.Exit(1)
	}
	port, err := lsCtr.MappedPort(ctx, "4566/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] LocalStack port: %v\n", err)
		os.Exit(1)
	}
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	repo, err := dynamodbrepo.New(ctx, endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] DynamoDB repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	fmt.Println("  [PASS] LocalStack container started")
	fmt.Println("  [PASS] DynamoDB connection established")
	fmt.Println("  [PASS] Table created")

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		fmt.Printf("  [FAIL] Seed data: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Seed data insertion")

	_, err = repo.LoadWindow(ctx, conv.ID, 1)
	if err != nil {
		fmt.Printf("  [FAIL] Sample query: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Sample query")
	fmt.Println("  DynamoDB dry run complete")
}

// runTurso connects to Turso Cloud, creates the repository, runs all scenarios,
// and prints results with BackendMeta. If isAll is true and env vars are missing,
// a warning is printed and execution skips (per D-14).
func runTurso(ctx context.Context, prof generator.Profile, selectedScenarios []benchmark.Scenario, iters, warmup, conc int, seed int64, isAll bool) {
	url := os.Getenv("TURSO_URL")
	authToken := os.Getenv("TURSO_AUTH_TOKEN")

	if url == "" || authToken == "" {
		if isAll {
			fmt.Println("Skipping turso: TURSO_URL and TURSO_AUTH_TOKEN not set")
			return
		}
		fmt.Fprintln(os.Stderr, "error: TURSO_URL and TURSO_AUTH_TOKEN must be set for --backend turso")
		os.Exit(1)
	}

	fmt.Println("Connecting to Turso Cloud...")
	repo, err := tursorepo.New(ctx, url, authToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create Turso repository: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	config := benchmark.RunConfig{
		Backend:     "turso",
		Warmup:      warmup,
		Iterations:  iters,
		Concurrency: conc,
		Profile:     prof,
		Seed:        seed,
	}

	runner := benchmark.NewRunner(repo, selectedScenarios, config)
	results, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "turso benchmark failed: %v\n", err)
		os.Exit(1)
	}

	meta := benchmark.BackendMeta{
		Name:      "turso",
		Transport: "libsql:// (remote, internet)",
		Note:      "Latency includes internet round-trip to Turso Cloud",
	}
	benchmark.PrintResults(meta, prof.Name, iters, seed, results)
}

// runTursoDryRun verifies Turso connectivity without running benchmarks.
// If isAll is true and env vars are missing, skips with warning (per D-14).
func runTursoDryRun(ctx context.Context, isAll bool) {
	fmt.Println("=== Turso dry-run ===")
	url := os.Getenv("TURSO_URL")
	authToken := os.Getenv("TURSO_AUTH_TOKEN")

	if url == "" || authToken == "" {
		if isAll {
			fmt.Println("  [SKIP] Turso: TURSO_URL and TURSO_AUTH_TOKEN not set")
			return
		}
		fmt.Fprintln(os.Stderr, "error: TURSO_URL and TURSO_AUTH_TOKEN must be set for --backend turso")
		os.Exit(1)
	}

	repo, err := tursorepo.New(ctx, url, authToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] Turso connection: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	fmt.Println("  [PASS] Turso connection established")
	fmt.Println("  [PASS] Schema applied")

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		fmt.Printf("  [FAIL] Seed data: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Seed data insertion")

	_, err = repo.LoadWindow(ctx, conv.ID, 1)
	if err != nil {
		fmt.Printf("  [FAIL] Sample query: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [PASS] Sample query")
	fmt.Println("  Turso dry run complete")
}
