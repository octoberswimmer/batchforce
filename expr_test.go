package batch

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	force "github.com/ForceCLI/force/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBulkSession is a mock implementation of BulkSession for testing
type MockBulkSession struct {
	mock.Mock
}

func (m *MockBulkSession) CancelableQueryAndSend(ctx context.Context, soql string, out chan<- force.ForceRecord, opts ...func(*force.QueryOptions)) error {
	args := m.Called(ctx, soql, out, opts)
	return args.Error(0)
}

func (m *MockBulkSession) CreateBulkJob(job force.JobInfo, requestOptions ...func(*http.Request)) (force.JobInfo, error) {
	args := m.Called(job, requestOptions)
	return args.Get(0).(force.JobInfo), args.Error(1)
}

func (m *MockBulkSession) AddBatchToJob(content string, job force.JobInfo) (force.BatchInfo, error) {
	args := m.Called(content, job)
	return args.Get(0).(force.BatchInfo), args.Error(1)
}

func (m *MockBulkSession) CloseBulkJobWithContext(ctx context.Context, jobID string) (force.JobInfo, error) {
	args := m.Called(ctx, jobID)
	return args.Get(0).(force.JobInfo), args.Error(1)
}

func (m *MockBulkSession) GetJobInfo(jobID string) (force.JobInfo, error) {
	args := m.Called(jobID)
	return args.Get(0).(force.JobInfo), args.Error(1)
}

func (m *MockBulkSession) GetBatches(jobID string) ([]force.BatchInfo, error) {
	args := m.Called(jobID)
	return args.Get(0).([]force.BatchInfo), args.Error(1)
}

func (m *MockBulkSession) RetrieveBulkBatchResults(jobID, batchID string) (force.BatchResult, error) {
	args := m.Called(jobID, batchID)
	return args.Get(0).(force.BatchResult), args.Error(1)
}

func (m *MockBulkSession) GetAbsoluteBytes(url string) ([]byte, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func TestFetchFunction(t *testing.T) {
	tests := []struct {
		name          string
		expression    string
		mockSetup     func(*MockBulkSession)
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name:       "fetch relative URL from Salesforce",
			expression: `{"FetchedData": fetch("/services/data/v58.0/sobjects")}`,
			mockSetup: func(m *MockBulkSession) {
				m.On("GetAbsoluteBytes", "/services/data/v58.0/sobjects").Return([]byte(`{"encoding":"UTF-8","sobjects":[]}`), nil)
			},
			expectedValue: `{"encoding":"UTF-8","sobjects":[]}`,
			expectError:   false,
		},
		{
			name:       "fetch relative URL without leading slash",
			expression: `{"FetchedData": fetch("services/data/v58.0/sobjects")}`,
			mockSetup: func(m *MockBulkSession) {
				m.On("GetAbsoluteBytes", "/services/data/v58.0/sobjects").Return([]byte(`{"encoding":"UTF-8","sobjects":[]}`), nil)
			},
			expectedValue: `{"encoding":"UTF-8","sobjects":[]}`,
			expectError:   false,
		},
		{
			name:          "fetch external URL not supported",
			expression:    `{"FetchedData": fetch("https://example.com/data")}`,
			mockSetup:     func(m *MockBulkSession) {},
			expectError:   true,
			errorContains: "external URLs not supported",
		},
		{
			name:       "fetch with base64 encoding",
			expression: `{"EncodedData": base64(fetch("/test/data"))}`,
			mockSetup: func(m *MockBulkSession) {
				m.On("GetAbsoluteBytes", "/test/data").Return([]byte("Hello World"), nil)
			},
			expectedValue: "SGVsbG8gV29ybGQ=",
			expectError:   false,
		},
		{
			name:       "fetch error from Salesforce",
			expression: `{"FetchedData": fetch("/invalid/endpoint")}`,
			mockSetup: func(m *MockBulkSession) {
				m.On("GetAbsoluteBytes", "/invalid/endpoint").Return(nil, fmt.Errorf("404 Not Found"))
			},
			expectError:   true,
			errorContains: "failed to fetch from Salesforce",
		},
		{
			name:       "fetch ContentDocument VersionData example",
			expression: `{"Base64Content": base64(fetch(record.ContentDocument.LatestPublishedVersion.VersionData))}`,
			mockSetup: func(m *MockBulkSession) {
				m.On("GetAbsoluteBytes", "/services/data/v58.0/sobjects/ContentVersion/068xx0000000001/VersionData").Return([]byte("PDF content here"), nil)
			},
			expectedValue: "UERGIGNvbnRlbnQgaGVyZQ==",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock session
			mockSession := new(MockBulkSession)
			tt.mockSetup(mockSession)

			// Create expression converter with mock session
			converter, err := exprConverter(tt.expression, nil, mockSession)
			assert.NoError(t, err, "Failed to create expression converter")

			// Create a dummy record for testing
			record := force.ForceRecord{
				"Id":   "001000000000000",
				"Name": "Test",
			}

			// Add ContentDocument data for the specific test case
			if tt.name == "fetch ContentDocument VersionData example" {
				record["ContentDocument"] = map[string]interface{}{
					"LatestPublishedVersion": map[string]interface{}{
						"VersionData": "/services/data/v58.0/sobjects/ContentVersion/068xx0000000001/VersionData",
					},
				}
			}

			// Execute the expression
			defer func() {
				if r := recover(); r != nil {
					if tt.expectError {
						assert.Contains(t, fmt.Sprint(r), tt.errorContains)
					} else {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			results := converter(record)

			if !tt.expectError {
				assert.Len(t, results, 1)
				// Check the appropriate field based on test case
				if tt.name == "fetch with base64 encoding" {
					assert.Equal(t, tt.expectedValue, results[0]["EncodedData"])
				} else if tt.name == "fetch ContentDocument VersionData example" {
					assert.Equal(t, tt.expectedValue, results[0]["Base64Content"])
				} else {
					assert.Equal(t, tt.expectedValue, results[0]["FetchedData"])
				}
			}

			// Verify all expected calls were made
			mockSession.AssertExpectations(t)
		})
	}
}

func TestExpressionFunctionsWithSession(t *testing.T) {
	// Test that exprFunctions properly accepts a session parameter
	mockSession := new(MockBulkSession)
	functions := exprFunctions(mockSession)

	// Verify that functions are returned
	assert.NotEmpty(t, functions, "exprFunctions should return expression options")
}
