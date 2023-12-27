package apex

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/octoberswimmer/apexfmt/parser"
)

// Validate that the apex parses successfully
func ValidateAnonymous(code []byte) error {
	apex := wrap(string(code))
	return Validate(apex)
}

type errorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

func (e *errorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	e.errors = append(e.errors, fmt.Sprintln("line "+strconv.Itoa(line)+":"+strconv.Itoa(column)+" "+msg))
}

type varListener struct {
	*parser.BaseApexParserListener
	vars []string
}

func (l *varListener) EnterLocalVariableDeclarationStatement(ctx *parser.LocalVariableDeclarationStatementContext) {
	fmt.Println("Entering local variable declaration statement:", ctx.GetText())
	for _, v := range ctx.LocalVariableDeclaration().VariableDeclarators().AllVariableDeclarator() {
		varName := v.Id().GetText()
		l.vars = append(l.vars, varName)
	}
}

func Validate(code []byte) error {
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
		return errors.New(strings.Join(e.errors, "\n"))
	}
	return nil
}

// Wrap anonymous apex in class and method because apexfmt doesn't
// support anonymous apex yet
func wrap(anon string) []byte {
	wrapped := []byte(fmt.Sprintf(`public class Temp {
		public void run() {
		%s
		}
	}`, anon))
	return wrapped
}
