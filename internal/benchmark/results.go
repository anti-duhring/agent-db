package benchmark

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// ScenarioResult holds the latency percentiles recorded for a single scenario
// during the measured iteration pass. All latency values are in microseconds (D-10).
type ScenarioResult struct {
	ScenarioName string
	P50          int64 // microseconds
	P95          int64 // microseconds
	P99          int64 // microseconds
	TotalCount   int64 // number of measured iterations
}

// formatLatency converts a raw microsecond value to a human-readable string.
// Values below 1000us are displayed as "Xus"; values >= 1000us are displayed
// as "X.XXms" (D-12).
func formatLatency(microseconds int64) string {
	if microseconds < 1000 {
		return fmt.Sprintf("%dus", microseconds)
	}
	return fmt.Sprintf("%.2fms", float64(microseconds)/1000.0)
}

// PrintResults writes a formatted latency table to stdout for the given run
// configuration and scenario results. Uses text/tabwriter for column alignment.
// Output format matches D-10: header line + tab-separated table.
func PrintResults(backend string, profile string, iterations int, seed int64, results []ScenarioResult) {
	fmt.Printf("Backend: %s | Profile: %s | Iterations: %d | Seed: %d\n",
		backend, profile, iterations, seed)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SCENARIO\tP50\tP95\tP99\tCOUNT")
	fmt.Fprintln(w, "--------\t---\t---\t---\t-----")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			r.ScenarioName,
			formatLatency(r.P50),
			formatLatency(r.P95),
			formatLatency(r.P99),
			r.TotalCount,
		)
	}
	w.Flush()
}
