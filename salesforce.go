package batch

import (
	"fmt"
	"strings"
	"time"

	force "github.com/ForceCLI/force/lib"
	"github.com/octoberswimmer/batchforce/soql"
	log "github.com/sirupsen/logrus"
)

func processRecordsWithSubQueries(query string, input <-chan force.ForceRecord, output chan<- force.ForceRecord, converter Converter) (err error) {
	defer func() {
		close(output)
		if r := recover(); r != nil {
			// Make sure sender isn't blocked waiting for us to read
			for range input {
			}
			err = fmt.Errorf("panic occurred: %s", r)
		}
	}()

	subQueryRelationships, err := soql.SubQueryRelationships(query)
	if err != nil {
		return fmt.Errorf("Failed to parse query for subqueries: %w", err)
	}

INPUT:
	for {
		select {
		case record, more := <-input:
			if !more {
				log.Info("Done processing input records")
				break INPUT
			}
			flattened, err := flattenRecord(record, subQueryRelationships)
			if err != nil {
				return fmt.Errorf("flatten failed: %w", err)
			}
			updates := converter(flattened)
			for _, update := range updates {
				output <- update
			}
		case <-time.After(1 * time.Second):
			log.Info("Waiting for record to convert")
		}
	}
	return err
}

// Replace subquery results with the records for the sub-query
func flattenRecord(r force.ForceRecord, subQueryRelationships map[string]bool) (force.ForceRecord, error) {
	if len(subQueryRelationships) == 0 {
		return r, nil
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
				return nil, fmt.Errorf("got possible incomplete results for " + k + " subquery. done is false, but all results should have been retrieved")
			}
			r[k] = records
		}
	}
	return r, nil
}
