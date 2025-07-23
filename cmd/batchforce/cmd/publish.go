package cmd

import (
	"fmt"
	"os"

	force "github.com/ForceCLI/force/lib"
	. "github.com/octoberswimmer/batchforce"

	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [flags] /event/<Event> <Expr>",
	Short: "publish Platform Events using the Pub/Sub API",
	Example: `
$ batchforce publish --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" /event/Account_Change__e '{Account__c: record.Id, Name: record.Name + " Copy"}'
$ batchforce publish --file accounts.csv /event/Account_Change__e '{Id: record.Id, Name: record.Name + " Copy"}'
	`,
	DisableFlagsInUseLine: false,
	Args:                  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		execution, err := getExecution(cmd, args)
		if err != nil {
			return err
		}
		channel := args[0]
		// Session is BulkSession; assert to *force.Force for Pub/Sub client
		fs, ok := execution.Session.(*force.Force)
		if !ok {
			return fmt.Errorf("cannot use session as *force.Force for publish")
		}
		execution.RecordWriter = PublishTo(fs, channel)
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
