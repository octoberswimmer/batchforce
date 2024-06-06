package cmd

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"

	"github.com/spf13/cobra"
)

var insertCmd = &cobra.Command{
	Use:   "insert [flags] <SObject> <Expr>",
	Short: "insert Salesforce records using the Bulk API",
	Example: `
$ batchforce insert --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Name: record.Name + " Copy"}'
$ batchforce insert --file accounts.csv Account '{Name: record.Name + " Copy"}'
	`,
	DisableFlagsInUseLine: false,
	Args:                  cobra.ExactValidArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		execution, err := getExecution(cmd, args)
		if err != nil {
			return err
		}
		execution.JobOptions = append(execution.JobOptions, Insert)
		errors := execution.RunContext(cmd.Context())
		if errors.NumberBatchesFailed() > 0 {
			fmt.Println(errors.NumberBatchesFailed(), "batch failures")
			os.Exit(1)
		}
		if errors.NumberRecordsFailed() > 0 {
			fmt.Println(errors.NumberRecordsFailed(), "record failures")
			os.Exit(1)
		}
		return nil
	},
}
