package batch

import (
	"fmt"
	"strings"

	force "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/query"
	"github.com/octoberswimmer/batchforce/soql"
)

func makeFlatteningConverter(query string, converter Converter) (Converter, error) {
	subQueryRelationships, err := soql.SubQueryRelationships(query)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query for subqueries: %w", err)
	}
	flatteningConverter := func(record force.ForceRecord) []force.ForceRecord {
		flattened := flattenRecord(record, subQueryRelationships)
		return converter(flattened)
	}
	return flatteningConverter, nil
}

// Replace subquery results with the records for the sub-query
func flattenRecord(r force.ForceRecord, subQueryRelationships soql.Relationships) force.ForceRecord {
	for k, v := range r {
		if v == nil {
			continue
		}
		if subRelationships, found := subQueryRelationships[soql.Relationship(strings.ToLower(k))]; found {
			subQueryResults := v.([]query.Record)
			records := make([]force.ForceRecord, 0, len(subQueryResults))
			for _, s := range subQueryResults {
				records = append(records, flattenRecord(s.Fields, subRelationships))
			}
			r[k] = records
		} else if m, ok := v.(map[string]any); ok {
			delete(m, "attributes")
			r[k] = m
		}
	}
	return r
}
