package report

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// PrintCostTable writes a formatted cost projection table to w using tabwriter.
// The scale assumptions are printed in the header row.
func PrintCostTable(w io.Writer, projections []BackendCostProjection, scale ScaleConfig) {
	fmt.Fprintf(w, "\nCost Projections (Scale: %d users x %d convos x %d msgs/day)\n",
		scale.Users, scale.ConvosPerUser, scale.MsgsPerDay)

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight)
	defer tw.Flush()

	fmt.Fprintln(tw, "BACKEND\tINSTANCE/PLAN\tCOMPUTE\tSTORAGE\tI/O\tTOTAL/MO\t")
	fmt.Fprintln(tw, "-------\t-------------\t-------\t-------\t---\t--------\t")

	for _, p := range projections {
		fmt.Fprintf(tw, "%s\t%s\t$%.2f\t$%.2f\t$%.2f\t$%.2f\t\n",
			p.Backend,
			p.InstanceOrPlan,
			p.MonthlyCompute,
			p.MonthlyStorage,
			p.MonthlyIO,
			p.MonthlyTotal,
		)
	}
}

// PrintScorecardTable writes a formatted operational complexity scorecard table
// to w using tabwriter. Scores are displayed as N/5.
func PrintScorecardTable(w io.Writer) {
	fmt.Fprintf(w, "\nOperational Complexity Scorecard (1=worst, 5=best)\n")

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight)
	defer tw.Flush()

	fmt.Fprintln(tw, "DIMENSION\tPOSTGRES\tDYNAMODB\tTURSO\t")
	fmt.Fprintln(tw, "---------\t--------\t--------\t-----\t")

	for _, row := range HardcodedScorecard {
		fmt.Fprintf(tw, "%s\t%d/5\t%d/5\t%d/5\t\n",
			row.Dimension, row.Postgres, row.DynamoDB, row.Turso)
	}
}
