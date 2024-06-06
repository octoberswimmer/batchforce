package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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
