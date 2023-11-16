package cmd

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"

	"github.com/spf13/cobra"
)

var upsertCmd = &cobra.Command{
	Use:   "upsert [flags] <SObject> <Expr>",
	Short: "upsert Salesforce records using the Bulk API",
	Example: `
$ batchforce upsert --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Name: record.Name + " Copy"}'
$ batchforce upsert --file accounts.csv Account '{Name: record.Name + " Copy"}'
	`,
	DisableFlagsInUseLine: false,
	Args:                  cobra.ExactValidArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		execution, err := getExecution(cmd, args)
		if err != nil {
			return err
		}
		externalId, _ := cmd.Flags().GetString("external-id")
		execution.JobOptions = append(execution.JobOptions, Upsert, ExternalId(externalId))
		errors := execution.RunContext(cmd.Context())
		if errors.NumberBatchesFailed > 0 {
			fmt.Println(errors.NumberBatchesFailed, "batch failures")
			os.Exit(1)
		}
		if errors.NumberRecordsFailed > 0 {
			fmt.Println(errors.NumberRecordsFailed, "record failures")
			os.Exit(1)
		}
		return nil
	},
}
