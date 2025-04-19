package cmd

import (
	"context"
	"fmt"
	"os"

	force "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/pubsub"
	. "github.com/octoberswimmer/batchforce"

	log "github.com/sirupsen/logrus"
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
	Args:                  cobra.ExactValidArgs(2),
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
		execution.RecordWriter = publishTo(fs, channel)
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

type uncheckableResult struct {
}

func (u uncheckableResult) NumberBatchesFailed() int {
	return 0
}

func (u uncheckableResult) NumberRecordsFailed() int {
	return 0
}

func publishTo(session *force.Force, channel string) RecordWriter {
	return func(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
		force.Log = log.StandardLogger()
		res := uncheckableResult{}
		err := pubsub.PublishMessagesWithContext(ctx, session, channel, records)
		return res, err
	}
}
