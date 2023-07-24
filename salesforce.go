package batch

import (
	"strings"

	force "github.com/ForceCLI/force/lib"
	"github.com/octoberswimmer/batchforce/soql"
	log "github.com/sirupsen/logrus"
)

func processRecordsWithSubQueries(query string, channels ProcessorChannels, converter Converter) {
	defer func() {
		close(channels.output)
		if err := recover(); err != nil {
			select {
			// Make sure sender isn't blocked waiting for us to read
			case <-channels.input:
			default:
			}
			log.Errorln("panic occurred:", err)
			log.Errorln("Sending abort signal")
			channels.abort <- true
		}
	}()

	subQueryRelationships, err := soql.SubQueryRelationships(query)
	if err != nil {
		panic("Failed to parse query for subqueries: " + err.Error())
	}

	for record := range channels.input {
		updates := converter(flattenRecord(record, subQueryRelationships))
		for _, update := range updates {
			channels.output <- update
		}
	}
}

// Replace subquery results with the records for the sub-query
func flattenRecord(r force.ForceRecord, subQueryRelationships map[string]bool) force.ForceRecord {
	if len(subQueryRelationships) == 0 {
		return r
	}
	for k, v := range r {
		if v == nil {
			continue
		}
		if _, found := subQueryRelationships[strings.ToLower(k)]; found {
			subQuery := v.(map[string]interface{})
			records := subQuery["records"].([]interface{})
			done := subQuery["done"].(bool)
			if !done {
				log.Fatalln("got possible incomplete reesults for " + k + " subquery. done is false, but all results should have been retrieved")
			}
			r[k] = records
		}
	}
	return r
}
