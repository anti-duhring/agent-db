package report

import (
	"encoding/json"
	"io"

	"github.com/anti-duhring/agent-db/internal/benchmark"
)

// ScenarioResultJSON is the JSON-serializable representation of a single
// scenario latency result.
type ScenarioResultJSON struct {
	Name  string `json:"name"`
	P50us int64  `json:"p50_us"`
	P95us int64  `json:"p95_us"`
	P99us int64  `json:"p99_us"`
	Count int64  `json:"count"`
}

// BackendResultJSON is the JSON-serializable representation of all results
// for a single backend.
type BackendResultJSON struct {
	Backend   string               `json:"backend"`
	Transport string               `json:"transport"`
	Note      string               `json:"note,omitempty"`
	Scenarios []ScenarioResultJSON `json:"scenarios"`
}

// BenchmarkReport is the top-level JSON envelope per D-11.
// It contains four sibling keys: metadata, results, cost_projections, scorecard.
type BenchmarkReport struct {
	Metadata        RunMetadata              `json:"metadata"`
	Results         []BackendResultJSON      `json:"results"`
	CostProjections []BackendCostProjection  `json:"cost_projections"`
	Scorecard       []ScorecardRow           `json:"scorecard"`
}

// BackendResults aggregates all results from a single backend run.
// This is the input type consumed by PrintJSON and GenerateMarkdown.
type BackendResults struct {
	Meta    benchmark.BackendMeta
	Results []benchmark.ScenarioResult
}

// PrintJSON serializes a complete benchmark report to w as indented JSON.
// The output is a single well-formed JSON object suitable for piping to jq.
// Per D-13: when --output json is used, this replaces human-readable tables.
func PrintJSON(w io.Writer, allResults []BackendResults, meta RunMetadata, projections []BackendCostProjection) error {
	backendResults := make([]BackendResultJSON, 0, len(allResults))
	for _, br := range allResults {
		scenarios := make([]ScenarioResultJSON, 0, len(br.Results))
		for _, r := range br.Results {
			scenarios = append(scenarios, ScenarioResultJSON{
				Name:  r.ScenarioName,
				P50us: r.P50,
				P95us: r.P95,
				P99us: r.P99,
				Count: r.TotalCount,
			})
		}
		backendResults = append(backendResults, BackendResultJSON{
			Backend:   br.Meta.Name,
			Transport: br.Meta.Transport,
			Note:      br.Meta.Note,
			Scenarios: scenarios,
		})
	}

	report := BenchmarkReport{
		Metadata:        meta,
		Results:         backendResults,
		CostProjections: projections,
		Scorecard:       HardcodedScorecard,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
