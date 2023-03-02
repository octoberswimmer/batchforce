package soql

import (
	"context"
	"fmt"
	"strings"

	"github.com/octoberswimmer/go-tree-sitter-sfapex/soql"
	log "github.com/sirupsen/logrus"
	sitter "github.com/smacker/go-tree-sitter"
)

func Validate(query []byte) error {
	n, err := sitter.ParseCtx(context.Background(), query, soql.GetLanguage())
	if err != nil {
		return err
	}
	if !n.HasError() {
		return nil
	}
	soqlError, err := getError(n, query)
	if err != nil {
		return fmt.Errorf("SOQL error: %w", err)
	}
	return fmt.Errorf("failed to parse soql: %s", soqlError)
}

// Get the Names of Relationships in Sub-Queries
func SubQueryRelationships(query string) (map[string]bool, error) {
	allRelationships := make(map[string]bool)
	q := []byte(query)
	err := Validate(q)
	if err != nil {
		return allRelationships, err
	}
	n, err := sitter.ParseCtx(context.Background(), q, soql.GetLanguage())
	if err != nil {
		return allRelationships, err
	}
	relationshipName := `
(select_clause
	(subquery
		(soql_query_body
			(from_clause
				(storage_identifier (identifier) @name)
		)
	 )
  )
)
`
	rels := make(map[string]bool)
	varQuery, err := sitter.NewQuery([]byte(relationshipName), soql.GetLanguage())
	if err != nil {
		return allRelationships, err
	}
	defer varQuery.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(varQuery, n)
	var lastNode *sitter.Node
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range match.Captures {
			lastNode = c.Node
			rels[lastNode.Content(q)] = true
		}
	}

	for k := range rels {
		allRelationships[strings.ToLower(k)] = true
	}

	return allRelationships, nil
}

func getError(node *sitter.Node, query []byte) (string, error) {
	e := `(ERROR) @node`
	errorQuery, err := sitter.NewQuery([]byte(e), soql.GetLanguage())
	if err != nil {
		log.Warnln("bad query for ERROR node", err.Error())
		return "", err
	}
	defer errorQuery.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(errorQuery, node)
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range match.Captures {
			v := c.Node.Content(query)
			return "Error at " + v, nil
		}
	}
	return "", fmt.Errorf("unknown error")
}
