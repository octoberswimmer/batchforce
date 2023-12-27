package apex

import (
	"errors"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/octoberswimmer/apexfmt/parser"
)

func Vars(anon string) ([]string, error) {
	code := wrap(anon)
	err := Validate(code)
	if err != nil {
		return nil, err
	}
	input := antlr.NewInputStream(string(code))
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewApexParser(stream)
	p.RemoveErrorListeners()
	e := &errorListener{}
	p.AddErrorListener(e)

	l := new(varListener)
	antlr.ParseTreeWalkerDefault.Walk(l, p.CompilationUnit())
	if len(e.errors) > 0 {
		return nil, errors.New(strings.Join(e.errors, "\n"))
	}

	return l.vars, nil
}
