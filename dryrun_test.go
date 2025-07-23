package batch

import (
	"bytes"
	"context"
	"strings"
	"testing"

	force "github.com/ForceCLI/force/lib"
)

func TestDryRunAddsNewlinesBetweenRecords(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create an execution with a custom PreviewWriter
	exec := &Execution{
		PreviewWriter: &buf,
	}

	// Create a channel with test records
	records := make(chan force.ForceRecord, 2)
	records <- force.ForceRecord{"Id": "001000000000001", "Name": "Test Account 1"}
	records <- force.ForceRecord{"Id": "001000000000002", "Name": "Test Account 2"}
	close(records)

	// Run dry run
	ctx := context.Background()
	_, err := exec.dryRun(ctx, records)
	if err != nil {
		t.Fatalf("dryRun returned error: %v", err)
	}

	// Check output
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly 2 lines
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d. Output:\n%s", len(lines), output)
	}

	// Each line should be valid JSON
	expectedLines := []string{
		`{"Id":"001000000000001","Name":"Test Account 1"}`,
		`{"Id":"001000000000002","Name":"Test Account 2"}`,
	}

	for i, line := range lines {
		if line != expectedLines[i] {
			t.Errorf("Line %d mismatch:\nExpected: %s\nGot:      %s", i+1, expectedLines[i], line)
		}
	}
}

func TestDryRunWithSingleRecord(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create an execution with a custom PreviewWriter
	exec := &Execution{
		PreviewWriter: &buf,
	}

	// Create a channel with a single test record
	records := make(chan force.ForceRecord, 1)
	records <- force.ForceRecord{"Id": "001000000000001", "Name": "Test Account"}
	close(records)

	// Run dry run
	ctx := context.Background()
	_, err := exec.dryRun(ctx, records)
	if err != nil {
		t.Fatalf("dryRun returned error: %v", err)
	}

	// Check output
	output := buf.String()

	// Should end with a newline
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("Output should end with a newline. Got: %q", output)
	}

	// Should have exactly one line when trimmed
	trimmed := strings.TrimSpace(output)
	if strings.Contains(trimmed, "\n") {
		t.Errorf("Output should contain only one record. Got: %q", output)
	}

	// Should be valid JSON
	expected := `{"Id":"001000000000001","Name":"Test Account"}`
	if trimmed != expected {
		t.Errorf("Output mismatch:\nExpected: %s\nGot:      %s", expected, trimmed)
	}
}
