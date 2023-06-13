package batch

import (
	b64 "encoding/base64"
	"html"
	"strings"

	"github.com/antonmedv/expr"
	strip "github.com/grokify/html-strip-tags-go"
)

func exprFunctions() []expr.Option {
	var exprFunctions []expr.Option
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

	store := make(map[string]any)
	// compareAndSet takes a key and value
	// returns true if the key exists in the store and the stored value matches the passed value
	// returns true if the key does not exist in the store, and stores the value under the key
	// returns false if the key exists in the store and the stored value does not match the passed value
	exprFunctions = append(exprFunctions, expr.Function(
		"compareAndSet",
		func(params ...any) (any, error) {
			key := params[0].(string)
			value := params[1]
			if store[key] == value {
				return true, nil
			} else if _, ok := store[key]; !ok {
				store[key] = value
				return true, nil
			} else {
				return false, nil
			}
		},
		new(func(string, any) bool),
	))

	counters := make(map[string]int64)
	// incr increments the number stored at key by one.  If the key does not
	// exist, it is set to 0 first.
	exprFunctions = append(exprFunctions, expr.Function(
		"incr",
		func(params ...any) (any, error) {
			key := params[0].(string)
			if v, ok := counters[key]; ok {
				counters[key] = v + 1
			} else {
				counters[key] = 0
			}
			return counters[key], nil
		},
		new(func(string) int64),
	))

	return exprFunctions
}
