package cmd

import (
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
		result := execution.RunContext(cmd.Context())
		if reportFailures(os.Stdout, result) {
			os.Exit(1)
		}
		return nil
	},
}
