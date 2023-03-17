package batch

import (
	b64 "encoding/base64"

	"github.com/antonmedv/expr"
	strip "github.com/grokify/html-strip-tags-go"
)

var base64 expr.Option
var stripHtml expr.Option

func init() {
	base64 = expr.Function(
		"base64",
		func(params ...any) (any, error) {
			return b64.StdEncoding.EncodeToString([]byte(params[0].(string))), nil
		},
		new(func(string) string),
	)

	stripHtml = expr.Function(
		"stripHtml",
		func(params ...any) (any, error) {
			return strip.StripTags(params[0].(string)), nil
		},
		new(func(string) string),
	)
}
