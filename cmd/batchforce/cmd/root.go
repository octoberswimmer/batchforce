package cmd

import (
	"fmt"
	"os"

	. "github.com/octoberswimmer/batchforce"

	force "github.com/ForceCLI/force/lib"
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
	RootCmd.PersistentFlags().Bool("quiet", false, "suppress informational log messages")
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
	- escapeUnicode: escapes characters using Unicode escape sequences
	  like Apex's String.escapeUnicode method
	- base64: base-64 encodes input
	- md5: md5 hash of string
	- getSet: set key to value, returning previous value for key
	- compareAndSet: check if key maps to value; if key doesn't exist, set it to
	  value (return true unless key already exists with different value)
	- changeValue: update value associated with key (returns true unless the key
	  already exists and the value is unchanged)
	- incr: increments the number stored at key by one. set to 1 if not set.
	- clone: create a copy of the record

	The + and - operators can be used to add, update, or remove fields on the
	record object.  For example:
	record + {RecordTypeId: apex.myRecordTypeId} - "RecordType.Name"

	If creating multiple records from a source record, use clone to avoid mutating
	the same object repeatedly.  For example:
	1..100 | map(clone(record) + {Name: "Record " + string(#)})

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
		if quiet, _ := cmd.Flags().GetBool("quiet"); quiet {
			log.SetLevel(log.WarnLevel)
		}
	},
	DisableFlagsInUseLine: true,
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
