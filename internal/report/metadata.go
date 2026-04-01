package report

import (
	"runtime/debug"
	"time"
)

// RunMetadata captures information about the benchmark run environment.
// All fields are serialized to JSON for machine-readable output (D-11, D-12).
type RunMetadata struct {
	Timestamp  string `json:"timestamp"`  // RFC3339 UTC timestamp when benchmark ran
	GitSHA     string `json:"git_sha"`    // VCS revision from runtime/debug (or "unknown" for dev builds)
	GoVersion  string `json:"go_version"` // Go toolchain version from runtime/debug
	Seed       int64  `json:"seed"`       // Benchmark random seed for reproducibility
	Profile    string `json:"profile"`    // Data profile used (small/medium/large)
	Iterations int    `json:"iterations"` // Number of measured iterations per scenario
}

// CollectMetadata builds RunMetadata by reading build info from runtime/debug.
// VCS fields will be "unknown" for development builds run with `go run` or
// builds without git history — this is expected and not an error.
func CollectMetadata(seed int64, profile string, iterations int) RunMetadata {
	goVer := "unknown"
	gitSHA := "unknown"

	if info, ok := debug.ReadBuildInfo(); ok {
		goVer = info.GoVersion
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				gitSHA = s.Value
			}
		}
		// dev builds have no vcs.revision — leave as "unknown"
	}

	return RunMetadata{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		GitSHA:     gitSHA,
		GoVersion:  goVer,
		Seed:       seed,
		Profile:    profile,
		Iterations: iterations,
	}
}
