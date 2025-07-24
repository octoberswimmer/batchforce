package batch

import (
	"encoding/json"
	"fmt"
	force "github.com/ForceCLI/force/lib"
	anon "github.com/octoberswimmer/batchforce/apex"
	"strings"
)

func (e *Execution) getApexContext() (map[string]any, error) {
	apex := e.Apex
	apexVars, err := anon.Vars(apex)
	if err != nil {
		return nil, err
	}
	lines := []string{"\n" + `Map<String, Object> b_f_c_t_x = new Map<String, Object>();`}
	for _, v := range apexVars {
		lines = append(lines, fmt.Sprintf(`b_f_c_t_x.put('%s', %s);`, v, v))
	}
	// Throw an exception with the JSON data to retrieve it via Tooling API
	lines = append(lines, `throw new System.HandledException('BATCHFORCE_CONTEXT:' + JSON.serialize(b_f_c_t_x));`)
	apex = apex + strings.Join(lines, "\n")

	// retrieve underlying *force.Force
	fs, ok := e.session().(*force.Force)
	if !ok {
		return nil, fmt.Errorf("session is not a *force.Force")
	}

	// Use Tooling API for ExecuteAnonymous (works in WASM)
	result, err := fs.ExecuteAnonymousTooling(apex)
	if err != nil {
		return nil, fmt.Errorf("failed to execute anonymous apex: %v", err)
	}

	// Check if we got our expected exception with the context data
	if !result.Success && result.ExceptionMessage != "" {
		// Look for our marker in the exception message (may have exception type prefix)
		marker := "BATCHFORCE_CONTEXT:"
		idx := strings.Index(result.ExceptionMessage, marker)
		if idx != -1 {
			// Extract the JSON from the exception message
			jsonStr := result.ExceptionMessage[idx+len(marker):]
			var n map[string]any
			err = json.Unmarshal([]byte(jsonStr), &n)
			if err != nil {
				return nil, fmt.Errorf("failed to parse context JSON: %v", err)
			}
			return n, nil
		}
		// This is a real exception, not our context data
		return nil, fmt.Errorf("execution error: %s", result.ExceptionMessage)
	}

	// If no exception was thrown (no variables to extract), return empty map
	if result.Success {
		return map[string]any{}, nil
	}

	// Handle compilation errors
	if result.CompileProblem != "" {
		return nil, fmt.Errorf("compilation error: %s", result.CompileProblem)
	}

	return nil, fmt.Errorf("execution failed")
}
