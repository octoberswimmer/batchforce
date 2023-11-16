package cmd

import (
	"fmt"
	"log"

	. "github.com/octoberswimmer/batchforce"

	"github.com/spf13/cobra"
)

func getExecution(cmd *cobra.Command, args []string) (*Execution, error) {
	var execution *Execution
	object := args[0]
	expr := args[1]

	query, _ := cmd.Flags().GetString("query-all")
	queryAll := query != ""
	if query == "" {
		query, _ = cmd.Flags().GetString("query")
	}
	csv, _ := cmd.Flags().GetString("file")

	if query == "" && csv == "" {
		return execution, fmt.Errorf("Either --query/--query-all or --file must be set")
	}

	var err error
	if csv != "" {
		execution, err = NewCSVExecution(object, csv)
		if err != nil {
			log.Fatalf("Could not initialize csv import: %s", err.Error())
		}
	} else {
		execution, err = NewQueryExecution(object, query)
		if err != nil {
			log.Fatalf("Could not initialize query: %s", err.Error())
		}
		execution.QueryAll = queryAll
	}

	execution.Expr = expr
	execution.Session = session
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		execution.DryRun = true
	}

	if batchSize, _ := cmd.Flags().GetInt("batch-size"); batchSize > 0 {
		execution.BatchSize = batchSize
	}
	if apex, _ := cmd.Flags().GetString("context"); apex != "" {
		execution.Apex = apex
	}

	var jobOptions []JobOption
	if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
		jobOptions = append(jobOptions, Serialize)
	}
	execution.JobOptions = jobOptions
	return execution, nil
}
