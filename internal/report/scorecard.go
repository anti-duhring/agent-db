package report

// ScorecardRow holds the operational complexity scores for one evaluation dimension.
// Scores are on a 1-5 scale where 1 = worst and 5 = best.
type ScorecardRow struct {
	Dimension string
	Postgres  int
	DynamoDB  int
	Turso     int
	Rationale string
}

// HardcodedScorecard contains operational complexity scores based on
// Phase 1-3 implementation experience. Scores reflect actual developer
// experience with each backend, not theoretical evaluation.
var HardcodedScorecard = []ScorecardRow{
	{
		Dimension: "SDK Ergonomics",
		Postgres:  5,
		DynamoDB:  3,
		Turso:     4,
		Rationale: "pgx idiomatic Go; DynamoDB expression builder verbose; Turso is standard sql.DB",
	},
	{
		Dimension: "Connection Management",
		Postgres:  4,
		DynamoDB:  5,
		Turso:     4,
		Rationale: "pgxpool requires pre-schema setup; DynamoDB stateless SDK; Turso sql.DB standard",
	},
	{
		Dimension: "Error Handling",
		Postgres:  4,
		DynamoDB:  3,
		Turso:     4,
		Rationale: "Postgres error codes clear; DynamoDB has service+marshaling errors layered; Turso standard sql errors",
	},
	{
		Dimension: "Schema Migration",
		Postgres:  5,
		DynamoDB:  2,
		Turso:     4,
		Rationale: "Postgres standard SQL DDL; DynamoDB no schema migrations; Turso SQL works but SQLite constraints",
	},
	{
		Dimension: "Local Dev Story",
		Postgres:  5,
		DynamoDB:  3,
		Turso:     2,
		Rationale: "Postgres testcontainers trivial; DynamoDB LocalStack adequate; Turso requires real internet",
	},
}
