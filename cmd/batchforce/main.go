package main

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	updateCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	updateCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	updateCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	insertCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	insertCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	insertCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	insertCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	RootCmd.AddCommand(updateCmd)
	RootCmd.AddCommand(insertCmd)
}

func main() {
	Execute()
}

var updateCmd = &cobra.Command{
	Use:   "update <SObject> <SOQL> <Expr>",
	Short: "Update Salesforce records using the Bulk API",
	Long: `
	Update Salesforce records using the Bulk API and a SOQL query.

	Optionally use anonymous apex to provide additional context.

	The SOQL query is used to generate the input.  Each record returned by the
	query is made available to the Expr expression as a map named "record".  See
	https://expr.medv.io/ for details on the Expr language.  The expression
	evaluate to an map of the form, "{ Field: Value, ... }".

	Additional context to be provided to the Expr expression by passing the
	--context parameter containining anonymous apex to execute before the
	records are queried.  Each apex variable defined will be available within
	the "apex" map.
	`,
	Example: `
$ batchforce update Account "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" '{Id: record.Id, Name: record.Name + " Test"}'

$ batchforce update Account "SELECT Id, Type__c FROM Account WHERE RecordType.DeveloperName = 'OldValue'" '{Id: record.Id, RecordTypeId: apex.recordTypes[record.Type__c]}' \
  --context "Map<String, Id> recordTypes = new Map<String, Id>(); for (RecordType r : [SELECT DeveloperName, Id FROM RecordType WHERE SobjectType = 'Account']){ recordTypes.put(r.DeveloperName, r.Id); }"
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

		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}
		if batchSize, _ := cmd.Flags().GetInt("batch-size"); batchSize > 0 {
			jobOptions = append(jobOptions, BatchSize(batchSize))
		}

		apex, _ := cmd.Flags().GetString("context")
		errors := RunExprWithApex(object, query, expr, apex, jobOptions...)
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

var insertCmd = &cobra.Command{
	Use:   "insert <SObject> <SOQL> <Expr>",
	Short: "insert Salesforce records using the Bulk API",
	Long: `
	Insert Salesforce records using the Bulk API and a SOQL query.

	Optionally use anonymous apex to provide additional context.

	The SOQL query is used to generate the input.  Each record returned by the
	query is made available to the Expr expression as a map named "record".  See
	https://expr.medv.io/ for details on the Expr language.  The expression
	evaluate to an map of the form, "{ Field: Value, ... }".

	Additional context to be provided to the Expr expression by passing the
	--context parameter containining anonymous apex to execute before the
	records are queried.  Each apex variable defined will be available within
	the "apex" map.
	`,
	Example: `
$ batchforce insert Account "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" '{Name: record.Name + " Copy"}'
	`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactValidArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		query := args[1]
		expr := args[2]

		jobOptions := []JobOption{Insert}

		if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
			jobOptions = append(jobOptions, DryRun)
		}

		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}
		if batchSize, _ := cmd.Flags().GetInt("batch-size"); batchSize > 0 {
			jobOptions = append(jobOptions, BatchSize(batchSize))
		}

		apex, _ := cmd.Flags().GetString("context")
		errors := RunExprWithApex(object, query, expr, apex, jobOptions...)
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
