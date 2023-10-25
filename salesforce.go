package batch

import (
	"fmt"
	"strings"

	force "github.com/ForceCLI/force/lib"
	"github.com/octoberswimmer/batchforce/soql"
)

func makeFlatteningConverter(query string, converter Converter) (Converter, error) {
	subQueryRelationships, err := soql.SubQueryRelationships(query)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query for subqueries: %w", err)
	}
	flatteningConverter := func(record force.ForceRecord) []force.ForceRecord {
		flattened, err := flattenRecord(record, subQueryRelationships)
		if err != nil {
			panic("Could not flatten record: " + err.Error())
		}
		return converter(flattened)
	}
	return flatteningConverter, nil
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
