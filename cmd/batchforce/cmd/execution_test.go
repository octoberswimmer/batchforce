package cmd

import (
	"bytes"
	"testing"

	force "github.com/ForceCLI/force/lib"
	. "github.com/octoberswimmer/batchforce"
)

func TestReportFailuresWritesEachFailedRecordAsJSON(t *testing.T) {
	result := BulkJobResult{
		JobInfo: force.JobInfo{NumberRecordsFailed: 1},
		Failures: []RecordFailure{{
			BatchId: "batch-1",
			Record:  force.ForceRecord{"External_Id__c": "B"},
			Errors: []force.ResultError{{
				StatusCode: "REQUIRED_FIELD_MISSING",
				Message:    "Required fields are missing: [LastName]",
				Fields:     []string{"LastName"},
			}},
		}},
	}
	var out bytes.Buffer

	if !reportFailures(&out, result) {
		t.Error("Expected failures to be reported")
	}

	expected := `{"batchId":"batch-1","record":{"External_Id__c":"B"},"errors":[{"statusCode":"REQUIRED_FIELD_MISSING","message":"Required fields are missing: [LastName]","fields":["LastName"]}]}
1 record failures
`
	if out.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, out.String())
	}
}

func TestReportFailuresReportsBatchAndRecordCounts(t *testing.T) {
	result := BulkJobResult{JobInfo: force.JobInfo{NumberBatchesFailed: 1, NumberRecordsFailed: 2}}
	var out bytes.Buffer

	if !reportFailures(&out, result) {
		t.Error("Expected failures to be reported")
	}

	expected := "1 batch failures\n2 record failures\n"
	if out.String() != expected {
		t.Errorf("Expected %q, got %q", expected, out.String())
	}
}

func TestReportFailuresWritesNothingWithoutFailures(t *testing.T) {
	var out bytes.Buffer

	if reportFailures(&out, BulkJobResult{}) {
		t.Error("Expected no failures")
	}
	if out.Len() != 0 {
		t.Errorf("Expected no output, got %q", out.String())
	}
}
