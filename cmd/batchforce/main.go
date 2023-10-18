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
	for _, cmd := range []*cobra.Command{updateCmd, insertCmd, upsertCmd, deleteCmd} {
		cmd.Flags().StringP("query", "q", "", "SOQL query for input data")
		cmd.Flags().String("query-all", "", "query all records (including archived/deleted records)")
		cmd.Flags().StringP("file", "f", "", "CSV file for input data")

		cmd.Flags().StringP("context", "c", "", "provide context with anonymous apex")

		cmd.Flags().BoolP("serialize", "s", false, "serial mode.  Run batch job in Serial mode (default: Parallel)")
		cmd.Flags().IntP("batch-size", "b", 0, "batch size.  Set batch size (default: 2000)")

		cmd.Flags().BoolP("dry-run", "n", false, "dry run.  Display updates without modifying records")
		cmd.MarkFlagsMutuallyExclusive("query", "query-all")
		cmd.MarkFlagsMutuallyExclusive("query", "file")
		cmd.MarkFlagsMutuallyExclusive("file", "query-all")
	}

	upsertCmd.Flags().StringP("external-id", "e", "", "external id")
	upsertCmd.MarkFlagRequired("external-id")

	deleteCmd.Flags().Bool("hard-delete", false, "hard delete records.  Bypass recycle bin and hard delete records")

	RootCmd.AddCommand(updateCmd)
	RootCmd.AddCommand(insertCmd)
	RootCmd.AddCommand(upsertCmd)
	RootCmd.AddCommand(deleteCmd)
	RootCmd.AddCommand(versionCmd)

	RootCmd.PersistentFlags().StringP("account", "a", "", "account `username` to use")
}

func main() {
	Execute()
}

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
		execution, err := getExecution(cmd, args)
		if err != nil {
			return err
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

var RootCmd = &cobra.Command{
	Use:   "batchforce",
	Short: "Use Bulk API to update Salesforce records",
	Long: `
	Insert/Update/Delete Salesforce records using the Bulk API and a SOQL query.

	Optionally use anonymous apex to provide additional context.

	The SOQL query is used to generate the input.  Each record returned by the
	query is made available to the Expr expression as a map named "record".  See
	https://expr.medv.io/ for details on the Expr language.  The expression should
	evaluate to an map of the form, "{ Field: Value, ... }" or an array of such
	maps.

	In addition to Expr's built-in operators and functions, the following
	functions can be used within the expression:
	- stripHtml: removes HTML tags
	- escapeHtml: escapes characters using HTML entities like Apex's
	  String.escapeHtml4 method
	- base64: base-64 encodes input
	- compareAndSet: check if key maps to value; if key doesn't exist, set it to
	  value
	- incr: increments the number stored at key by one. set to 0 if not set.

	The + and - operators can be used to add, update, or remove fields on the
	record object.  For example:
	record + {RecordTypeId: apex.myRecordTypeId} - "RecordType.Name"

	Additional context to be provided to the Expr expression by passing the
	--context parameter containining anonymous apex to execute before the
	records are queried.  Each apex variable defined will be available within
	the "apex" map.

	A csv file can be used as input instead of a SOQL query by using the --file
	parameter.  This is often useful when combined with --apex to map input to
	org-specific values such as Record Type Ids.
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display current version",
	Example: `
  batchforce version
`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
