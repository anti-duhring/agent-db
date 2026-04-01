package benchmark

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// ScenarioResult holds the latency percentiles and iteration count for a
// single scenario run.
type ScenarioResult struct {
	ScenarioName string
	P50          int64 // microseconds
	P95          int64 // microseconds
	P99          int64 // microseconds
	TotalCount   int64 // number of measured iterations
}

// BackendMeta carries per-backend metadata displayed in the results header (per D-15, D-16).
type BackendMeta struct {
	Name      string // e.g., "postgres", "dynamodb", "turso"
	Transport string // e.g., "pgx/v5 (local container)", "aws-sdk-go-v2 (LocalStack)", "libsql:// (remote, internet)"
	Note      string // optional, e.g., "Latency includes internet round-trip to Turso Cloud"
}

// formatLatency converts a microsecond latency value to a human-readable string.
// Values under 1ms are displayed as microseconds; values >= 1ms as milliseconds.
func formatLatency(microseconds int64) string {
	if microseconds < 1000 {
		return fmt.Sprintf("%dus", microseconds)
	}
	return fmt.Sprintf("%.2fms", float64(microseconds)/1000.0)
}

// PrintResults prints a formatted results table to stdout showing p50/p95/p99
// latency for each scenario. BackendMeta provides the transport and note lines.
func PrintResults(meta BackendMeta, profile string, iterations int, seed int64, results []ScenarioResult) {
	fmt.Printf("\nBackend: %s | Transport: %s | Profile: %s | Iterations: %d | Seed: %d\n",
		meta.Name, meta.Transport, profile, iterations, seed)
	if meta.Note != "" {
		fmt.Printf("Note: %s\n", meta.Note)
	}
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "SCENARIO\tP50\tP95\tP99\tCOUNT\t")
	fmt.Fprintln(w, "--------\t---\t---\t---\t-----\t")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t\n",
			r.ScenarioName,
			formatLatency(r.P50),
			formatLatency(r.P95),
			formatLatency(r.P99),
			r.TotalCount,
		)
	}
	w.Flush()
}
