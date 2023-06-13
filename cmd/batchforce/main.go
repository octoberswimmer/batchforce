package main

import (
	"fmt"
	"os"

	force "github.com/ForceCLI/force/lib"
	. "github.com/octoberswimmer/batchforce"
	"github.com/spf13/cobra"
)

var session *force.Force

func init() {
	updateCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	updateCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	updateCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	updateCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	insertCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	insertCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	insertCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	insertCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	deleteCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	deleteCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	deleteCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	deleteCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	RootCmd.AddCommand(updateCmd)
	RootCmd.AddCommand(insertCmd)
	RootCmd.AddCommand(deleteCmd)

	RootCmd.PersistentFlags().StringP("account", "a", "", "account `username` to use")
}

func main() {
	Execute()
}

var updateCmd = &cobra.Command{
	Use:   "update <SObject> <SOQL> <Expr>",
	Short: "Update Salesforce records using the Bulk API",
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

		execution := NewExecution(object, query)
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
		errors := execution.Run()
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
	Example: `
$ batchforce insert Account "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" '{Name: record.Name + " Copy"}'
	`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactValidArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		query := args[1]
		expr := args[2]

		execution := NewExecution(object, query)
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

		jobOptions := []JobOption{Insert}
		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}
		execution.JobOptions = jobOptions
		errors := execution.Run()
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

var deleteCmd = &cobra.Command{
	Use:   "delete <SObject> <SOQL> [Expr]",
	Short: "delete Salesforce records using the Bulk API",
	Example: `
$ batchforce delete Account "SELECT Id FROM Account WHERE Name LIKE '%test'"
$ batchforce delete Account "SELECT Id, Name FROM Account" 'record.Name matches "\d{3,5}" ? {Id: record.Id} : nil'
	`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		query := args[1]
		var expr string
		if len(args) == 3 {
			expr = args[2]
		} else {
			expr = "{Id: record.Id}"
		}

		execution := NewExecution(object, query)
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

		jobOptions := []JobOption{Delete}
		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}

		execution.JobOptions = jobOptions
		errors := execution.Run()
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
	Long: `
	Insert/Update/Delete Salesforce records using the Bulk API and a SOQL query.

	Optionally use anonymous apex to provide additional context.

	The SOQL query is used to generate the input.  Each record returned by the
	query is made available to the Expr expression as a map named "record".  See
	https://expr.medv.io/ for details on the Expr language.  The expression
	evaluate to an map of the form, "{ Field: Value, ... }".

	In addition to Expr's built-in operators and functions, the following
	functions can be used within the expression:
	- stripHtml: removes HTML tags
	- escapeHtml: escapes characters using HTML entities like Apex's
	  String.escapeHtml4 method
	- base64: base-64 encodes input
	- compareAndSet: check if key maps to value; if key doesn't exist, set it to
	  value
	- incr: increments the number stored at key by one. set to 0 if not set.


	Additional context to be provided to the Expr expression by passing the
	--context parameter containining anonymous apex to execute before the
	records are queried.  Each apex variable defined will be available within
	the "apex" map.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initializeSession(cmd)
	},
	DisableFlagsInUseLine: true,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initializeSession(cmd *cobra.Command) {
	var err error
	if account, _ := cmd.Flags().GetString("account"); account != "" {
		session, err = force.GetForce(account)
	} else {
		session, err = force.ActiveForce()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not initialize session: "+err.Error())
		os.Exit(1)
	}
}
