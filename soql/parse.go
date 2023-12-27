package soql

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/octoberswimmer/apexfmt/parser"
)

type errorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

func (e *errorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	e.errors = append(e.errors, fmt.Sprintln("line "+strconv.Itoa(line)+":"+strconv.Itoa(column)+" "+msg))
}

type relationshipListener struct {
	*parser.BaseApexParserListener
	relationships map[string]bool
}

func newRelationshipListener() *relationshipListener {
	return &relationshipListener{
		relationships: make(map[string]bool),
	}
}

func (l *relationshipListener) EnterSubQuery(ctx *parser.SubQueryContext) {
	for _, v := range ctx.FromNameList().AllFieldNameAlias() {
		relName := ""
		if alias := v.SoqlId(); alias != nil {
			relName = alias.GetText()
		} else {
			relName = v.FieldName().GetText()
		}
		l.relationships[strings.ToLower(relName)] = true
	}
}

func Validate(query []byte) error {
	input := antlr.NewInputStream(string(query))
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewApexParser(stream)
	p.RemoveErrorListeners()
	e := &errorListener{}
	p.AddErrorListener(e)

	l := newRelationshipListener()
	antlr.ParseTreeWalkerDefault.Walk(l, p.Query())
	if len(e.errors) > 0 {
		return errors.New(strings.Join(e.errors, "\n"))
	}
	return nil
}

// Get the Names of Relationships in Sub-Queries
func SubQueryRelationships(query string) (map[string]bool, error) {
	input := antlr.NewInputStream(string(query))
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewApexParser(stream)
	p.RemoveErrorListeners()
	e := &errorListener{}
	p.AddErrorListener(e)

	l := newRelationshipListener()
	antlr.ParseTreeWalkerDefault.Walk(l, p.Query())
	if len(e.errors) > 0 {
		return nil, errors.New(strings.Join(e.errors, "\n"))
	}
	return l.relationships, nil
}
