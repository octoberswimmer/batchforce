package batch

import (
	"crypto/md5"
	b64 "encoding/base64"
	"fmt"
	"html"
	"strings"
	"unicode"

	force "github.com/ForceCLI/force/lib"
	"github.com/expr-lang/expr"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

type Env map[string]any

func (Env) MergePatch(a force.ForceRecord, b map[string]any) force.ForceRecord {
	for k, v := range b {
		a[k] = v
	}
	return a
}

func (Env) DeleteKey(a force.ForceRecord, b string) force.ForceRecord {
	delete(a, b)
	return a
}

// Escape any non-ASCII characters like Apex's String.escapeUnicode.
func escapeUnicode(s string) string {
	var result strings.Builder
	for _, rune := range s {
		if rune > unicode.MaxASCII {
			result.WriteString(fmt.Sprintf("\\u%04X", rune))
		} else {
			result.WriteRune(rune)
		}
	}
	return result.String()
}

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
		"escapeUnicode",
		func(params ...any) (any, error) {
			return escapeUnicode(params[0].(string)), nil
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

	exprFunctions = append(exprFunctions, expr.Function(
		"md5",
		func(params ...any) (any, error) {
			return fmt.Sprintf("%x", md5.Sum([]byte(params[0].(string)))), nil
		},
		new(func(string) string),
	))

	compareAndSetStore := make(map[string]any)
	// compareAndSet takes a key and value
	// returns true if the key exists in the store and the stored value matches the passed value
	// returns true if the key does not exist in the store, and stores the value under the key
	// returns false if the key exists in the store and the stored value does not match the passed value
	exprFunctions = append(exprFunctions, expr.Function(
		"compareAndSet",
		func(params ...any) (any, error) {
			key := params[0].(string)
			value := params[1]
			if compareAndSetStore[key] == value {
				return true, nil
			} else if _, ok := compareAndSetStore[key]; !ok {
				compareAndSetStore[key] = value
				return true, nil
			} else {
				return false, nil
			}
		},
		new(func(string, any) bool),
	))

	changeValueStore := make(map[string]any)
	// changeValue takes a key and value
	// returns true if the key exists did not already exist
	// returns true if the key already exists in the store, and the new value is different that the previous value
	// returns false if the key exists in the store and the stored value is the same as passed value
	exprFunctions = append(exprFunctions, expr.Function(
		"changeValue",
		func(params ...any) (any, error) {
			key := params[0].(string)
			value := params[1]
			if changeValueStore[key] == value {
				return false, nil
			}
			changeValueStore[key] = value
			return true, nil
		},
		new(func(string, any) bool),
	))

	getSetStore := make(map[string]any)
	// getSet takes a key and value
	// sets the current key to the new value
	// returns the previous value for the key
	exprFunctions = append(exprFunctions, expr.Function(
		"getSet",
		func(params ...any) (any, error) {
			key := params[0].(string)
			value := params[1]
			prevValue, ok := getSetStore[key]
			getSetStore[key] = value
			if ok {
				return prevValue, nil
			}
			switch v := value.(type) {
			case int:
				return int(0), nil
			case float64:
				return float64(0), nil
			case []string:
				return []string{}, nil
			case []any:
				return []any{}, nil
			case string:
				return "", nil
			default:
				return prevValue, fmt.Errorf("Unsupported type %T", v)
			}
		},
		new(func(string, int) int),
		new(func(string, float64) float64),
		new(func(string, string) string),
		new(func(string, []string) []string),
		new(func(string, []any) []any),
	))

	counters := make(map[string]int64)
	// incr increments the number stored at key by one.  If the key does not
	// exist, it is set to 1.
	exprFunctions = append(exprFunctions, expr.Function(
		"incr",
		func(params ...any) (any, error) {
			key := params[0].(string)
			if v, ok := counters[key]; ok {
				counters[key] = v + 1
			} else {
				counters[key] = 1
			}
			return counters[key], nil
		},
		new(func(string) int64),
	))

	exprFunctions = append(exprFunctions, expr.Operator("+", "MergePatch"))
	exprFunctions = append(exprFunctions, expr.Operator("-", "DeleteKey"))

	return exprFunctions
}

func exprConverter(expression string, context any) func(force.ForceRecord) []force.ForceRecord {
	env := Env{
		"record": force.ForceRecord{},
		"apex":   context,
	}
	program, err := expr.Compile(expression, append(exprFunctions(), expr.Env(env))...)
	if err != nil {
		log.Fatalln("Invalid expression:", err)
	}
	converter := func(record force.ForceRecord) []force.ForceRecord {
		env := Env{
			"record": record,
			"apex":   context,
		}
		out, err := expr.Run(program, env)
		if err != nil {
			panic(err)
		}
		if out == nil {
			return []force.ForceRecord{}
		}
		var singleRecord force.ForceRecord
		err = mapstructure.Decode(out, &singleRecord)
		if err == nil {
			return []force.ForceRecord{singleRecord}
		}
		var multipleRecords []force.ForceRecord
		err = mapstructure.Decode(out, &multipleRecords)
		if err == nil {
			return multipleRecords
		}
		log.Warnln("Unexpected value.  It should be a map or array or maps.  Got", out)
		return []force.ForceRecord{}
	}
	return converter
}
