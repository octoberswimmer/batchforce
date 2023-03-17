package batch

import (
	b64 "encoding/base64"

	"github.com/antonmedv/expr"
)

var base64 expr.Option

func init() {
	base64 = expr.Function(
		"base64",
		func(params ...any) (any, error) {
			return b64.StdEncoding.EncodeToString([]byte(params[0].(string))), nil
		},
		new(func(string) string),
	)
}
