package main

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")
	RootCmd.AddCommand(updateCmd)
}

func main() {
	Execute()
}

var updateCmd = &cobra.Command{
	Use:   "update <SObject> <SOQL> <Expr>",
	Short: "Update Salesforce records using the Bulk API",
	Example: `
$ batchforce update Account "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" '{Id: record.Id, Name: record.Name + " Test"}'
	`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactValidArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		query := args[1]
		expr := args[2]

		var jobOptions []JobOption
		if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
			jobOptions = append(jobOptions, DryRun)
		}
		errors := RunExpr(object, query, expr, jobOptions...)
		if errors.NumberBatchesFailed > 0 {
			fmt.Println(errors.NumberBatchesFailed, "batch failures")
			os.Exit(1)
		}
		if errors.NumberRecordsFailed > 0 {
			fmt.Println(errors.NumberRecordsFailed, "record failures")
			os.Exit(1)
		}
	},
}

var RootCmd = &cobra.Command{
	Use:   "batchforce",
	Short: "Use Bulk API to update Salesforce records",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
	DisableFlagsInUseLine: true,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
