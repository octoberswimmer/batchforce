package main

import (
	"fmt"
	force "github.com/ForceCLI/force/lib"
	"octoberswimmer/batchforce"
	"strings"
)

var guestLookups = []string{
	"Guest_1__c",
	"Guest_Look_up_1__c",
	"Guest_Look_up_2__c",
	"Guest_Look_up_3__c",
	"Guest_Look_up_4__c",
	"Guest_Look_up_5__c",
	"Guest_Look_up_6__c",
	"Guest_Look_up_7__c",
	"Guest_Look_up_8__c",
	"Guest_Look_up_9__c",
	"Guest_Look_up_10__c",
	"Guest_Look_up_11__c",
	"Guest_Look_up_12__c",
}

func main() {
	query := fmt.Sprintf(`
		SELECT
			Id,
			%s
		FROM
			Hospitality__c
		WHERE
			Id NOT IN (SELECT Hospitality__c FROM Visit__c)
	`, strings.Join(guestLookups, ",\n"))

	batchforce.Run("Visit__c", query, makeVisits, func(job *batchforce.BulkJob) {
		job.Operation = "insert"
	})
}

func makeVisits(hospitality force.ForceRecord) (visits []force.ForceRecord) {
	// Keep track of "<account id>-<hospitality id>" so we don't create multiple visits if a guest
	// is on the same hospitality multiple times.
	seenVisits := make(map[string]bool, 128)

	for _, field := range guestLookups {
		accountId, guestFieldPresent := hospitality[field]
		if !guestFieldPresent {
			continue
		}
		uniqueVisitId := fmt.Sprintf("%s-%s", accountId, hospitality["Id"])
		if alreadySeen := seenVisits[uniqueVisitId]; alreadySeen {
			continue
		}
		seenVisits[uniqueVisitId] = true
		visit := force.ForceRecord{
			"Account__c": accountId,
			"Hospitality__c": hospitality["Id"],
		}
		visits = append(visits, visit)
	}
	return
}
