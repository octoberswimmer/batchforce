package batch

import (
	b64 "encoding/base64"
	"html"
	"strings"

	"github.com/antonmedv/expr"
	strip "github.com/grokify/html-strip-tags-go"
)

var exprFunctions []expr.Option

func init() {
	exprFunctions = append(exprFunctions, expr.Function(
		"base64",
		func(params ...any) (any, error) {
			return b64.StdEncoding.EncodeToString([]byte(params[0].(string))), nil
		},
		new(func(string) string),
	))

	exprFunctions = append(exprFunctions, expr.Function(
		"stripHtml",
		func(params ...any) (any, error) {
			return strip.StripTags(params[0].(string)), nil
		},
		new(func(string) string),
	))

	exprFunctions = append(exprFunctions, expr.Function(
		"escapeHtml",
		func(params ...any) (any, error) {
			return strings.ReplaceAll(html.EscapeString(params[0].(string)), `&#34;`, `&quot;`), nil
		},
		new(func(string) string),
	))

}
