package report

// CostConfig holds pricing defaults for all three database backends.
// All values are hardcoded defaults based on current published pricing
// (verified 2026-03-31) and can be overridden via CLI flags.
type CostConfig struct {
	// DynamoDB on-demand pricing (us-east-1)
	// Source: aws.amazon.com/dynamodb/pricing/on-demand/ — verified 2026-03-31
	// Note: Prices were reduced in November 2024 from $0.78 to $0.25/million WRU
	DynamoDBWRUPerMillion float64 // Default: 0.25 — $0.25 per million WRU
	DynamoDBRRUPerMillion float64 // Default: 0.125 — $0.125 per million RRU
	DynamoDBStoragePerGB  float64 // Default: 0.25 — $0.25 per GB-month (Standard class)

	// DynamoDB WRU cost per AppendMessage call.
	// TransactWriteItems 4 items (message + old listing delete + new listing + meta update)
	// x 2 WRU each in on-demand mode = 8 WRU per message append (per Phase 3 D-04).
	DynamoDBWRUPerAppend int // Default: 8

	// RDS PostgreSQL pricing (us-east-1, on-demand)
	// Source: aws.amazon.com/rds/postgresql/pricing/ — verified 2026-03-31
	RDSInstanceHourly float64 // Default: 0.030 — db.t4g.micro on-demand hourly rate
	RDSInstanceType   string  // Default: "db.t4g.micro"
	RDSStoragePerGB   float64 // Default: 0.115 — gp3 storage, us-east-1
	RDSHoursPerMonth  int     // Default: 730 — standard billing month

	// Turso plan pricing
	// Source: turso.tech/pricing — verified 2026-03-31 (pricing changes frequently; verify before use)
	TursoStarterMonthly   float64 // Default: 0 — free starter tier
	TursoStarterRowReads  int64   // Default: 500,000,000 — 500M row reads/month on starter
	TursoStarterRowWrites int64   // Default: 10,000,000 — 10M row writes/month on starter
	TursoScalerMonthly    float64 // Default: 29 — $29/month scaler plan
	TursoScalerRowReads   int64   // Default: 100,000,000,000 — 100B row reads/month on scaler
	TursoScalerRowWrites  int64   // Default: 100,000,000 — 100M row writes/month on scaler
	TursoOverageReadPerBillion  float64 // Default: 1.00 — $1.00 per billion row reads over limit
	TursoOverageWritePerMillion float64 // Default: 1.00 — $1.00 per million row writes over limit
}

// DefaultCostConfig returns a CostConfig with all hardcoded pricing defaults.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		DynamoDBWRUPerMillion:       0.25,
		DynamoDBRRUPerMillion:       0.125,
		DynamoDBStoragePerGB:        0.25,
		DynamoDBWRUPerAppend:        8,
		RDSInstanceHourly:           0.030,
		RDSInstanceType:             "db.t4g.micro",
		RDSStoragePerGB:             0.115,
		RDSHoursPerMonth:            730,
		TursoStarterMonthly:         0,
		TursoStarterRowReads:        500_000_000,
		TursoStarterRowWrites:       10_000_000,
		TursoScalerMonthly:          29,
		TursoScalerRowReads:         100_000_000_000,
		TursoScalerRowWrites:        100_000_000,
		TursoOverageReadPerBillion:  1.00,
		TursoOverageWritePerMillion: 1.00,
	}
}

// ScaleConfig defines the projected usage at the scale the cost model is evaluated.
type ScaleConfig struct {
	Users         int // Number of concurrent users
	ConvosPerUser int // Average conversations per user
	MsgsPerDay    int // Average messages per user per day
}

// DefaultScaleConfig returns the default scale: 100 users x 50 convos x 200 msgs/day.
func DefaultScaleConfig() ScaleConfig {
	return ScaleConfig{
		Users:         100,
		ConvosPerUser: 50,
		MsgsPerDay:    200,
	}
}

// BackendCostProjection holds the computed monthly cost breakdown for one backend.
type BackendCostProjection struct {
	Backend        string  // "postgres", "dynamodb", or "turso"
	InstanceOrPlan string  // e.g., "db.t4g.micro", "on-demand", "starter", "scaler"
	MonthlyCompute float64 // Instance/plan base cost
	MonthlyStorage float64 // Storage cost
	MonthlyIO      float64 // I/O request cost (DynamoDB WRU/RRU; RDS: 0)
	MonthlyTotal   float64 // Sum of all monthly costs
	Notes          string  // Human-readable notes about assumptions
}

// ComputeProjections calculates monthly cost projections for all three backends
// given a scale configuration and pricing configuration.
func ComputeProjections(scale ScaleConfig, cost CostConfig) []BackendCostProjection {
	return []BackendCostProjection{
		computeDynamoDB(scale, cost),
		computePostgres(scale, cost),
		computeTurso(scale, cost),
	}
}

func computeDynamoDB(scale ScaleConfig, cost CostConfig) BackendCostProjection {
	dailyWrites := scale.Users * scale.ConvosPerUser * scale.MsgsPerDay
	monthlyWRU := int64(dailyWrites) * int64(cost.DynamoDBWRUPerAppend) * 30
	monthlyWRUCost := float64(monthlyWRU) / 1_000_000 * cost.DynamoDBWRUPerMillion

	// Reads: LoadWindow + ListConversations = ~2 reads per write
	dailyReads := dailyWrites * 2
	monthlyRRU := int64(dailyReads) * 30
	monthlyRRUCost := float64(monthlyRRU) / 1_000_000 * cost.DynamoDBRRUPerMillion

	// Storage: rough estimate using 2KB average message size, 12 months accumulation
	estimatedStorageGB := float64(int64(dailyWrites)*30*12) * 0.002 / 1024
	storageCost := estimatedStorageGB * cost.DynamoDBStoragePerGB

	monthlyIO := monthlyWRUCost + monthlyRRUCost
	total := monthlyIO + storageCost

	return BackendCostProjection{
		Backend:        "dynamodb",
		InstanceOrPlan: "on-demand",
		MonthlyCompute: 0, // No instance cost for on-demand DynamoDB
		MonthlyStorage: storageCost,
		MonthlyIO:      monthlyIO,
		MonthlyTotal:   total,
		Notes: "AppendMessage uses TransactWriteItems (4 items x 2 WRU = 8 WRU/msg). " +
			"Reads estimated at 2x daily writes (LoadWindow + ListConversations). " +
			"Storage estimated at 2KB/msg, 12 months accumulation.",
	}
}

func computePostgres(scale ScaleConfig, cost CostConfig) BackendCostProjection {
	instanceCost := float64(cost.RDSHoursPerMonth) * cost.RDSInstanceHourly

	dailyWrites := scale.Users * scale.ConvosPerUser * scale.MsgsPerDay
	estimatedStorageGB := float64(int64(dailyWrites)*30*12) * 0.002 / 1024
	storageCost := estimatedStorageGB * cost.RDSStoragePerGB

	total := instanceCost + storageCost

	return BackendCostProjection{
		Backend:        "postgres",
		InstanceOrPlan: cost.RDSInstanceType,
		MonthlyCompute: instanceCost,
		MonthlyStorage: storageCost,
		MonthlyIO:      0, // RDS storage I/O included in gp3 storage cost
		MonthlyTotal:   total,
		Notes: "RDS on-demand instance cost + gp3 storage. " +
			"Storage estimated at 2KB/msg, 12 months accumulation. " +
			"No per-request I/O billing with gp3.",
	}
}

func computeTurso(scale ScaleConfig, cost CostConfig) BackendCostProjection {
	dailyWrites := scale.Users * scale.ConvosPerUser * scale.MsgsPerDay
	dailyReads := dailyWrites * 2

	// Turso bills per scanned row, not logical query.
	// AppendMessage = ~3 row writes (INSERT message + UPDATE listing + UPDATE meta)
	monthlyRowWrites := int64(dailyWrites) * 3 * 30
	// LoadWindow scans 20 rows; ListConversations scans 2x returned rows (Pitfall 5)
	monthlyRowReads := int64(dailyReads) * 30 * 20

	var planName string
	var planCost float64
	var overage float64

	// Determine tier based on projected usage
	if monthlyRowReads <= cost.TursoStarterRowReads && monthlyRowWrites <= cost.TursoStarterRowWrites {
		planName = "starter"
		planCost = cost.TursoStarterMonthly
	} else if monthlyRowReads <= cost.TursoScalerRowReads && monthlyRowWrites <= cost.TursoScalerRowWrites {
		planName = "scaler"
		planCost = cost.TursoScalerMonthly
	} else {
		// Exceeds scaler limits — compute overage
		planName = "scaler+overage"
		planCost = cost.TursoScalerMonthly

		excessReads := monthlyRowReads - cost.TursoScalerRowReads
		if excessReads > 0 {
			overage += float64(excessReads) / 1_000_000_000 * cost.TursoOverageReadPerBillion
		}

		excessWrites := monthlyRowWrites - cost.TursoScalerRowWrites
		if excessWrites > 0 {
			overage += float64(excessWrites) / 1_000_000 * cost.TursoOverageWritePerMillion
		}
	}

	total := planCost + overage

	return BackendCostProjection{
		Backend:        "turso",
		InstanceOrPlan: planName,
		MonthlyCompute: planCost,
		MonthlyStorage: 0, // Turso storage included in plan
		MonthlyIO:      overage,
		MonthlyTotal:   total,
		Notes: "Turso bills per scanned row. Row writes estimated at 3/msg (INSERT + UPDATE listing + UPDATE meta). " +
			"Row reads estimated at 20/query (LoadWindow 20-row scan + ListConversations 2x multiplier). " +
			"Storage included in plan cost.",
	}
}
