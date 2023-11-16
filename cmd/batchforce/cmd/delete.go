package cmd

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [flags] <SObject> [Expr]",
	Short: "delete Salesforce records using the Bulk API",
	Example: `
$ batchforce delete --query "SELECT Id FROM Account WHERE Name LIKE Account '%test'"
$ batchforce delete --query "SELECT Id, Name FROM Account" Account 'record.Name matches "\d{3,5}" ? {Id: record.Id} : nil'
	`,
	DisableFlagsInUseLine: false,
	Args:                  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			args = append(args, "{Id: record.Id}")
		}
		execution, err := getExecution(cmd, args)
		if err != nil {
			return err
		}
		if hardDelete, _ := cmd.Flags().GetBool("hard-delete"); hardDelete {
			execution.JobOptions = append(execution.JobOptions, HardDelete)
		} else {
			execution.JobOptions = append(execution.JobOptions, Delete)
		}
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
