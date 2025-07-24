package batch

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	b64 "encoding/base64"
	"encoding/gob"
	"fmt"
	"html"
	"math/big"
	"os"
	"strings"
	"time"
	"unicode"

	force "github.com/ForceCLI/force/lib"
	"github.com/expr-lang/expr"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

func init() {
	gob.Register(force.ForceRecord{})
}

type Env struct {
	Record      force.ForceRecord `expr:"record"`
	ApexContext any               `expr:"apex"`
}

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

func exprFunctions(session BulkSession) []expr.Option {
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

	exprFunctions = append(exprFunctions, expr.Function(
		"clone",
		func(params ...any) (any, error) {
			source := params[0].(force.ForceRecord)
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			dec := gob.NewDecoder(&buf)
			err := enc.Encode(source)
			if err != nil {
				return nil, err
			}
			var copy force.ForceRecord
			err = dec.Decode(&copy)
			if err != nil {
				return nil, err
			}
			return copy, nil
		},
		new(func(force.ForceRecord) force.ForceRecord),
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

	exprFunctions = append(exprFunctions, expr.Function(
		"rand",
		func(params ...any) (any, error) {
			max := params[0].(int64)
			bigInt, err := rand.Int(rand.Reader, big.NewInt(max))
			if err != nil {
				return bigInt, err
			}
			return bigInt.Int64(), nil
		},
		new(func(int64) int64),
	))

	exprFunctions = append(exprFunctions, expr.Function(
		"readfile",
		func(params ...any) (any, error) {
			path := params[0].(string)
			content, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(content), nil
		},
		new(func(string) string),
	))

	// Same as Expr's builtin date function, but with support for Salesforce's
	// standard DateTime format.
	exprFunctions = append(exprFunctions, expr.Function(
		"date",
		func(args ...any) (any, error) {
			tz, ok := args[0].(*time.Location)
			if ok {
				args = args[1:]
			}

			date := args[0].(string)
			if len(args) == 2 {
				layout := args[1].(string)
				if tz != nil {
					return time.ParseInLocation(layout, date, tz)
				}
				return time.Parse(layout, date)
			}
			if len(args) == 3 {
				layout := args[1].(string)
				timeZone := args[2].(string)
				tz, err := time.LoadLocation(timeZone)
				if err != nil {
					return nil, err
				}
				t, err := time.ParseInLocation(layout, date, tz)
				if err != nil {
					return nil, err
				}
				return t, nil
			}

			layouts := []string{
				"2006-01-02T15:04:05-0700",
				"2006-01-02",
				"15:04:05",
				"2006-01-02 15:04:05",
				time.RFC3339,
				time.RFC822,
				time.RFC850,
				time.RFC1123,
			}
			for _, layout := range layouts {
				if tz == nil {
					t, err := time.Parse(layout, date)
					if err == nil {
						return t, nil
					}
				} else {
					t, err := time.ParseInLocation(layout, date, tz)
					if err == nil {
						return t, nil
					}
				}
			}
			return nil, fmt.Errorf("invalid date %s", date)
		},
	))

	// fetch retrieves content from a URL. If the URL is relative, it's fetched from the Salesforce instance.
	exprFunctions = append(exprFunctions, expr.Function(
		"fetch",
		func(params ...any) (any, error) {
			url := params[0].(string)

			var content []byte
			var err error

			// Check if URL is absolute or relative
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				// Absolute URL - fetch directly
				// Note: This will only work for URLs that don't require authentication
				// For authenticated external URLs, you'd need to implement custom logic
				return nil, fmt.Errorf("external URLs not supported in fetch function")
			} else {
				// Relative URL - fetch from Salesforce instance
				// Ensure the URL starts with /
				if !strings.HasPrefix(url, "/") {
					url = "/" + url
				}

				content, err = session.GetAbsoluteBytes(url)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch from Salesforce: %w", err)
				}
			}

			// Return as string for now - can be used with base64() function
			return string(content), nil
		},
		new(func(string) string),
	))

	exprFunctions = append(exprFunctions, expr.Operator("+", "MergePatch"))
	exprFunctions = append(exprFunctions, expr.Operator("-", "DeleteKey"))

	return exprFunctions
}

func exprConverter(expression string, context any, session BulkSession) (func(force.ForceRecord) []force.ForceRecord, error) {
	program, err := expr.Compile(expression, append(exprFunctions(session), expr.Env(Env{}))...)
	if err != nil {
		return nil, fmt.Errorf("Invalid expression: %w", err)
	}
	converter := func(record force.ForceRecord) []force.ForceRecord {
		env := Env{
			Record:      record,
			ApexContext: context,
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
	return converter, nil
}
