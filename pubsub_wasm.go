//go:build wasm

package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	force "github.com/ForceCLI/force/lib"
	log "github.com/sirupsen/logrus"
)

type uncheckableResult struct {
}

func (u uncheckableResult) NumberBatchesFailed() int {
	return 0
}

func (u uncheckableResult) NumberRecordsFailed() int {
	return 0
}

// PublishTo creates a RecordWriter that publishes platform events using REST API (WASM compatible)
func PublishTo(session *force.Force, eventChannel string) RecordWriter {
	return func(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
		res := uncheckableResult{}

		// Collect records to batch publish
		var events []force.ForceRecord
		for record := range records {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			default:
				events = append(events, record)

				// Batch publish every 200 events (Salesforce limit)
				if len(events) >= 200 {
					if err := publishBatch(session, eventChannel, events); err != nil {
						return res, err
					}
					events = events[:0] // Clear slice but keep capacity
				}
			}
		}

		// Publish any remaining events
		if len(events) > 0 {
			if err := publishBatch(session, eventChannel, events); err != nil {
				return res, err
			}
		}

		return res, nil
	}
}

func publishBatch(session *force.Force, eventChannel string, events []force.ForceRecord) error {
	if len(events) == 0 {
		return nil
	}

	// Platform events need to specify the event type
	eventType := eventChannel
	if strings.HasPrefix(eventType, "/event/") {
		eventType = strings.TrimPrefix(eventType, "/event/")
	}

	// Ensure each record has the attributes field with the correct type
	recordsWithAttributes := make([]map[string]interface{}, len(events))
	for i, event := range events {
		record := make(map[string]interface{})
		// Copy all fields from the event
		for k, v := range event {
			record[k] = v
		}
		// Ensure attributes field exists with the type
		if _, hasAttrs := record["attributes"]; !hasAttrs {
			record["attributes"] = map[string]string{"type": eventType}
		}
		recordsWithAttributes[i] = record
	}

	// Construct the composite request body
	compositeRequest := map[string]interface{}{
		"allOrNone": false,
		"records":   recordsWithAttributes,
	}

	// PostAbsolute expects a relative URL starting with /services/...
	endpoint := fmt.Sprintf("/services/data/%s/composite/sobjects", force.ApiVersion())

	body, err := json.Marshal(compositeRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %v", err)
	}

	log.Infof("Publishing %d events to %s", len(events), eventChannel)

	// Use the Force client's PostAbsolute method for raw JSON
	responseStr, err := session.PostAbsolute(endpoint, string(body))
	if err != nil {
		if err == force.SessionExpiredError {
			// Try to refresh session
			if refreshErr := session.RefreshSession(); refreshErr != nil {
				return fmt.Errorf("session expired and refresh failed: %v", refreshErr)
			}
			// Retry after refresh
			responseStr, err = session.PostAbsolute(endpoint, string(body))
			if err != nil {
				return fmt.Errorf("failed to publish events after session refresh: %v", err)
			}
		} else {
			return fmt.Errorf("failed to publish events: %v", err)
		}
	}

	responseBody := []byte(responseStr)

	// Parse the response to check for errors
	var results []struct {
		Id      string                   `json:"id"`
		Success bool                     `json:"success"`
		Errors  []map[string]interface{} `json:"errors"`
	}

	if err := json.Unmarshal(responseBody, &results); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for any failed events
	var failedCount int
	for i, result := range results {
		if !result.Success {
			failedCount++
			log.Errorf("Failed to publish event %d: %v", i, result.Errors)
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d events failed to publish", failedCount)
	}

	log.Infof("Successfully published %d events", len(events))
	return nil
}
