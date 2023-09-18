package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	force "github.com/ForceCLI/force/lib"
	. "github.com/octoberswimmer/batchforce"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var session *force.Force

func init() {
	updateCmd.Flags().StringP("query", "q", "", "SOQL query for input data")
	updateCmd.Flags().StringP("file", "f", "", "CSV file for input data")
	updateCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	updateCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	updateCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	updateCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	insertCmd.Flags().StringP("query", "q", "", "SOQL query for input data")
	insertCmd.Flags().StringP("file", "f", "", "CSV file for input data")
	insertCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	insertCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	insertCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	insertCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	upsertCmd.Flags().StringP("query", "q", "", "SOQL query for input data")
	upsertCmd.Flags().StringP("file", "f", "", "CSV file for input data")
	upsertCmd.Flags().StringP("external-id", "e", "", "external id")
	upsertCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	upsertCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	upsertCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	upsertCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	upsertCmd.MarkFlagRequired("external-id")

	deleteCmd.Flags().StringP("query", "q", "", "SOQL query for input data")
	deleteCmd.Flags().StringP("file", "f", "", "CSV file for input data")
	deleteCmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")
	deleteCmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
	deleteCmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")
	deleteCmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")

	RootCmd.AddCommand(updateCmd)
	RootCmd.AddCommand(insertCmd)
	RootCmd.AddCommand(upsertCmd)
	RootCmd.AddCommand(deleteCmd)

	RootCmd.PersistentFlags().StringP("account", "a", "", "account `username` to use")
}

func main() {
	Execute()
}

var updateCmd = &cobra.Command{
	Use:   "update [flags] <SObject> <Expr>",
	Short: "Update Salesforce records using the Bulk API",
	Example: `
$ batchforce update --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Id: record.Id, Name: record.Name + " Test"}'

$ batchforce update --query "SELECT Id, Type__c FROM Account WHERE RecordType.DeveloperName = 'OldValue'" Account '{Id: record.Id, RecordTypeId: apex.recordTypes[record.Type__c]}' \
  --context "Map<String, Id> recordTypes = new Map<String, Id>(); for (RecordType r : [SELECT DeveloperName, Id FROM RecordType WHERE SobjectType = 'Account']){ recordTypes.put(r.DeveloperName, r.Id); }"
	`,
	DisableFlagsInUseLine: false,
	Args:                  cobra.ExactValidArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		object := args[0]
		expr := args[1]

		query, _ := cmd.Flags().GetString("query")
		csv, _ := cmd.Flags().GetString("file")

		if query == "" && csv == "" {
			return fmt.Errorf("Either --query or --file must be set")
		}

		var execution *Execution
		var err error
		if csv != "" {
			execution, err = NewCSVExecution(object, csv)
		} else {
			execution, err = NewQueryExecution(object, query)
		}
		if err != nil {
			log.Fatalf("Could not initialize query: %s", err.Error())
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
		object := args[0]
		expr := args[1]

		query, _ := cmd.Flags().GetString("query")
		csv, _ := cmd.Flags().GetString("file")

		if query == "" && csv == "" {
			return fmt.Errorf("Either --query or --file must be set")
		}

		var execution *Execution
		var err error
		if csv != "" {
			execution, err = NewCSVExecution(object, csv)
		} else {
			execution, err = NewQueryExecution(object, query)
		}
		if err != nil {
			log.Fatalf("Could not initialize query: %s", err.Error())
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

		jobOptions := []JobOption{Insert}
		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}
		execution.JobOptions = jobOptions
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
		object := args[0]
		expr := args[1]

		externalId, _ := cmd.Flags().GetString("external-id")

		query, _ := cmd.Flags().GetString("query")
		csv, _ := cmd.Flags().GetString("file")

		if query == "" && csv == "" {
			return fmt.Errorf("Either --query or --file must be set")
		}

		var execution *Execution
		var err error
		if csv != "" {
			execution, err = NewCSVExecution(object, csv)
		} else {
			execution, err = NewQueryExecution(object, query)
		}
		if err != nil {
			log.Fatalf("Could not initialize query: %s", err.Error())
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

		jobOptions := []JobOption{Upsert, ExternalId(externalId)}
		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}
		execution.JobOptions = jobOptions
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
		object := args[0]
		var expr string
		if len(args) == 2 {
			expr = args[1]
		} else {
			expr = "{Id: record.Id}"
		}

		query, _ := cmd.Flags().GetString("query")
		csv, _ := cmd.Flags().GetString("file")

		if query == "" && csv == "" {
			return fmt.Errorf("Either --query or --file must be set")
		}

		var execution *Execution
		var err error
		if csv != "" {
			execution, err = NewCSVExecution(object, csv)
		} else {
			execution, err = NewQueryExecution(object, query)
		}
		if err != nil {
			log.Fatalf("Could not initialize query: %s", err.Error())
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

		jobOptions := []JobOption{Delete}
		if serialize, _ := cmd.Flags().GetBool("serialize"); serialize {
			jobOptions = append(jobOptions, Serialize)
		}

		execution.JobOptions = jobOptions
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

	The + operator can be used to add or update a field on the record object.

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

func cancelUponSignal(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		interuptsReceived := 0
		for {
			<-sigs
			if interuptsReceived > 0 {
				os.Exit(1)
			}
			log.Println("signal received.  cancelling.")
			cancel()
			interuptsReceived++
		}
	}()
}

func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	cancelUponSignal(cancel)
	if err := RootCmd.ExecuteContext(ctx); err != nil {
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
