package apex

import (
	"context"
	"fmt"
	"os"

	sfapex "github.com/octoberswimmer/go-tree-sitter-sfapex/apex"
	sitter "github.com/smacker/go-tree-sitter"
)

func LastVar(anon string) (string, error) {
	code := wrap(anon)
	err := Validate(code)
	if err != nil {
		return "", err
	}
	n, err := sitter.ParseCtx(context.Background(), code, sfapex.GetLanguage())
	if err != nil {
		return "", err
	}
	newVar := `(local_variable_declaration
		type: [
			(type_identifier)
			(generic_type)
		]
		declarator: (variable_declarator
			name: (identifier) @name
		)
	)`
	vars := make(map[string]bool)
	varQuery, err := sitter.NewQuery([]byte(newVar), sfapex.GetLanguage())
	if err != nil {
		return "", err
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
			vars[lastNode.Content(code)] = true
		}
	}
	update := `(expression_statement [
			(assignment_expression left: (identifier) @name)
			(method_invocation object: (identifier) @name)
		])`
	updateQuery, err := sitter.NewQuery([]byte(update), sfapex.GetLanguage())
	if err != nil {
		return "", err
	}
	defer updateQuery.Close()
	qc.Exec(updateQuery, n)
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range match.Captures {
			v := c.Node.Content(code)
			if _, ok := vars[v]; ok && c.Node.EndByte() > lastNode.EndByte() {
				lastNode = c.Node
			}
		}
	}

	lastVar := ""
	if lastNode != nil {
		lastVar = lastNode.Content(code)
	}

	return lastVar, nil
}

func Vars(anon string) ([]string, error) {
	var allVars []string
	code := wrap(anon)
	err := Validate(code)
	if err != nil {
		return allVars, err
	}
	n, err := sitter.ParseCtx(context.Background(), code, sfapex.GetLanguage())
	if err != nil {
		return allVars, err
	}
	newVar := `(local_variable_declaration
		type: [
			(type_identifier)
			(generic_type)
		]
		declarator: (variable_declarator
			name: (identifier) @name
		)
	)`
	vars := make(map[string]bool)
	varQuery, err := sitter.NewQuery([]byte(newVar), sfapex.GetLanguage())
	if err != nil {
		return allVars, err
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
			vars[lastNode.Content(code)] = true
		}
	}

	for k := range vars {
		allVars = append(allVars, k)
	}

	return allVars, nil
}

func printTree(n *sitter.Node, code []byte) {
	tree := sitter.NewTreeCursor(n)
	defer tree.Close()

	rootNode := tree.CurrentNode()
Tree:
	for {
		switch {
		case tree.CurrentFieldName() == "":
		default:
			fmt.Fprintf(os.Stderr, "TYPE: %s\nFIELD NAME: %s\nNODE: %s\nCONTENT: %s\n\n", tree.CurrentNode().Type(), tree.CurrentFieldName(), tree.CurrentNode().String(), tree.CurrentNode().Content([]byte(code)))
		}
		ok := tree.GoToFirstChild()
		if !ok {
			ok := tree.GoToNextSibling()
			if !ok {
			Sibling:
				for {
					tree.GoToParent()
					if tree.CurrentNode() == rootNode {
						break Tree
					}
					if tree.GoToNextSibling() {
						break Sibling
					}
				}
			}
		}
	}
}
