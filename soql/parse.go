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

type Relationship string
type Relationships map[Relationship]Relationships

func (e *errorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	e.errors = append(e.errors, fmt.Sprintln("line "+strconv.Itoa(line)+":"+strconv.Itoa(column)+" "+msg))
}

type relationshipListener struct {
	*parser.BaseApexParserListener
	relationships Relationships

	currentRelationships  *Relationships
	previousRelationships *Relationships
}

func newRelationshipListener() *relationshipListener {
	l := &relationshipListener{
		relationships: make(Relationships),
	}
	l.currentRelationships = &l.relationships
	return l
}

func (l *relationshipListener) EnterSubQuery(ctx *parser.SubQueryContext) {
	for _, v := range ctx.FromNameList().AllFieldNameAlias() {
		relName := ""
		if alias := v.SoqlId(); alias != nil {
			relName = alias.GetText()
		} else {
			relName = v.FieldName().GetText()
		}
		rel := make(Relationships)
		(*l.currentRelationships)[Relationship(strings.ToLower(relName))] = rel
		l.previousRelationships = l.currentRelationships
		l.currentRelationships = &rel
	}
}

func (l *relationshipListener) ExitSubQuery(ctx *parser.SubQueryContext) {
	l.currentRelationships = l.previousRelationships
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
func SubQueryRelationships(query string) (Relationships, error) {
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
