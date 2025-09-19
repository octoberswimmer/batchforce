package batch

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	force "github.com/ForceCLI/force/lib"
)

// fakeSession implements BulkSession for testing, blocking on CloseBulkJobWithContext until allowed.
type fakeSession struct {
	closedInvoked chan struct{}
	closeAllow    chan struct{}
	onAddBatch    func()

	// For monitoring simulation
	mu                  sync.Mutex
	totalBatches        int
	completedBatches    int
	jobState            string
	getJobInfoCallCount int

	// Custom GetJobInfo handler for tests
	customGetJobInfo func(jobID string) (force.JobInfo, error)
}

func newFakeSession() *fakeSession {
	return &fakeSession{
		closedInvoked: make(chan struct{}, 1),
		closeAllow:    make(chan struct{}),
		jobState:      "Open",
	}
}

// TestExecuteContext_DoesNotCloseBeforeAllRecords ensures the bulk job isn't closed
// until all records have been sent and processed.
func TestExecuteContext_DoesNotCloseBeforeAllRecords(t *testing.T) {
	fake := newFakeSession()
	// Track batches added
	batchCount := 0
	batchMutex := &sync.Mutex{}
	fake.onAddBatch = func() {
		batchMutex.Lock()
		batchCount++
		batchMutex.Unlock()
	}

	// recordSender sends two records, delaying the second
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		out <- force.ForceRecord{"Id": "1"}
		// simulate delay before sending second record
		time.Sleep(100 * time.Millisecond)
		out <- force.ForceRecord{"Id": "2"}
		close(out)
		return nil
	}
	// simple identity converter
	converter := func(r force.ForceRecord) []force.ForceRecord {
		return []force.ForceRecord{r}
	}
	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}
	// Allow close to proceed immediately since we're running synchronously
	close(fake.closeAllow)

	// run ExecuteContext and wait for completion
	_, err := e.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext failed: %v", err)
	}

	// Verify that both records were sent as batches before the job was closed
	batchMutex.Lock()
	finalBatchCount := batchCount
	batchMutex.Unlock()

	if finalBatchCount != 2 {
		t.Fatalf("Expected 2 batches to be added, but got %d", finalBatchCount)
	}

	// Verify that close was called
	select {
	case <-fake.closedInvoked:
		// Good, close was called
	default:
		t.Fatal("CloseBulkJobWithContext was not called")
	}
}

func (f *fakeSession) CancelableQueryAndSend(ctx context.Context, soql string, out chan<- force.ForceRecord, opts ...func(*force.QueryOptions)) error {
	// not used in RecordSender mode
	panic("CancelableQueryAndSend should not be called")
}

// CreateBulkJob matches BulkSession signature, accepts optional requestOptions
func (f *fakeSession) CreateBulkJob(job force.JobInfo, _ ...func(*http.Request)) (force.JobInfo, error) {
	return force.JobInfo{Id: "job-id", ContentType: "JSON"}, nil
}

func (f *fakeSession) AddBatchToJob(content string, job force.JobInfo) (force.BatchInfo, error) {
	f.mu.Lock()
	f.totalBatches++
	f.mu.Unlock()

	if f.onAddBatch != nil {
		f.onAddBatch()
	}
	return force.BatchInfo{Id: fmt.Sprintf("batch-%d", f.totalBatches)}, nil
}

func (f *fakeSession) CloseBulkJobWithContext(ctx context.Context, jobID string) (force.JobInfo, error) {
	// signal that CloseBulkJob was invoked
	f.closedInvoked <- struct{}{}

	f.mu.Lock()
	f.jobState = "Closed"
	f.completedBatches = f.totalBatches
	f.mu.Unlock()

	// block until allowed
	<-f.closeAllow

	f.mu.Lock()
	defer f.mu.Unlock()

	// return final job info with completed batch counts
	return force.JobInfo{
		Id:                     jobID,
		ContentType:            "JSON",
		State:                  f.jobState,
		NumberBatchesTotal:     f.totalBatches,
		NumberBatchesCompleted: f.completedBatches,
		NumberBatchesFailed:    0,
	}, nil
}

func (f *fakeSession) GetJobInfo(jobID string) (force.JobInfo, error) {
	// Use custom handler if provided (don't lock for custom handlers)
	if f.customGetJobInfo != nil {
		f.mu.Lock()
		f.getJobInfoCallCount++
		f.mu.Unlock()
		return f.customGetJobInfo(jobID)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.getJobInfoCallCount++

	// If job was closed, mark all batches as complete
	if f.jobState == "Closed" {
		f.completedBatches = f.totalBatches
	}

	return force.JobInfo{
		Id:                     jobID,
		ContentType:            "JSON",
		State:                  f.jobState,
		NumberBatchesTotal:     f.totalBatches,
		NumberBatchesCompleted: f.completedBatches,
		NumberBatchesFailed:    0,
		NumberRecordsProcessed: f.completedBatches * 100, // Simulate records processed
		NumberRecordsFailed:    0,
	}, nil
}

func (f *fakeSession) GetBatches(jobID string) ([]force.BatchInfo, error) {
	return nil, nil
}

func (f *fakeSession) RetrieveBulkBatchResults(jobID, batchID string) (force.BatchResult, error) {
	return force.BatchResult{}, nil
}

func (f *fakeSession) GetAbsoluteBytes(url string) ([]byte, error) {
	return nil, fmt.Errorf("GetAbsoluteBytes not implemented in fakeSession")
}

// TestExecuteContext_WaitsForClose ensures ExecuteContext does not return until CloseBulkJob completes, even if context is canceled.
func TestExecuteContext_WaitsForClose(t *testing.T) {
	fake := newFakeSession()
	// recordSender sends one record then closes
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		out <- force.ForceRecord{"Id": "1"}
		close(out)
		return nil
	}
	// simple identity converter
	converter := func(r force.ForceRecord) []force.ForceRecord { return []force.ForceRecord{r} }
	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var res Result
	var err error
	go func() {
		res, err = e.ExecuteContext(ctx)
		close(done)
	}()
	// wait for CloseBulkJobWithContext invocation
	select {
	case <-fake.closedInvoked:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("CloseBulkJobWithContext was not called")
	}
	// cancel the context
	cancel()
	// ensure ExecuteContext is still blocking
	select {
	case <-done:
		t.Fatal("ExecuteContext returned before CloseBulkJob finished")
	case <-time.After(50 * time.Millisecond):
		// expected: still waiting
	}
	// allow close to complete
	close(fake.closeAllow)
	// now ExecuteContext should finish
	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ExecuteContext did not return after CloseBulkJob finished")
	}
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	if res.NumberBatchesFailed() != 0 {
		t.Errorf("unexpected NumberBatchesFailed: %d", res.NumberBatchesFailed())
	}
}

// TestExecuteContext_CanceledBeforeStart ensures ExecuteContext returns immediately with context.Canceled when canceled before any work, and no bulk job is started.
func TestExecuteContext_CanceledBeforeStart(t *testing.T) {
	fake := newFakeSession()
	// sender will wait, then close channel without sending records
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		// delay to simulate work and allow cancellation
		time.Sleep(50 * time.Millisecond)
		close(out)
		return nil
	}
	converter := func(r force.ForceRecord) []force.ForceRecord { return []force.ForceRecord{r} }
	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var res Result
	var err error
	go func() {
		res, err = e.ExecuteContext(ctx)
		close(done)
	}()
	// cancel while sender is sleeping, before job starts
	time.Sleep(10 * time.Millisecond)
	cancel()
	// wait for ExecuteContext to return
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ExecuteContext did not return after cancellation before job start")
	}
	// should return early with a cancellation error
	if err == nil {
		t.Fatal("expected error when cancel before job start, got nil")
	}
	if !strings.Contains(err.Error(), "Processing canceled") {
		t.Fatalf("expected Processing canceled error, got %v", err)
	}
	select {
	case <-fake.closedInvoked:
		t.Fatal("CloseBulkJobWithContext should not be called when canceled before start")
	default:
	}
	// result should be nil when canceled before job start
	if res != nil {
		t.Fatalf("expected nil result when cancel before job start, got %v", res)
	}
}

// TestExecuteContext_ConverterPanic ensures a panic in the converter is caught and returned as an error.
func TestExecuteContext_ConverterPanic(t *testing.T) {
	fake := newFakeSession()
	// sender will send two records then close
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		out <- force.ForceRecord{"Id": "1"}
		out <- force.ForceRecord{"Id": "2"}
		close(out)
		return nil
	}
	// converter panics on second record
	count := 0
	converter := func(r force.ForceRecord) []force.ForceRecord {
		count++
		if count > 1 {
			panic("boom")
		}
		return []force.ForceRecord{r}
	}
	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}
	// run ExecuteContext in background so we can unblock CloseBulkJob
	ctx := context.Background()
	done := make(chan struct{})
	var err error
	go func() {
		_, err = e.ExecuteContext(ctx)
		close(done)
	}()
	// wait for CloseBulkJob to be invoked
	select {
	case <-fake.closedInvoked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected bulk job to be closed after converter panic")
	}
	// allow CloseBulkJob to return
	close(fake.closeAllow)
	// wait for ExecuteContext to finish
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ExecuteContext did not return after CloseBulkJob")
	}
	if err == nil || !strings.Contains(err.Error(), "panic occurred") {
		t.Fatalf("expected panic error, got %v", err)
	}
}

// TestExecuteContext_MonitoringStartsAfterFirstBatch ensures that monitoring begins
// as soon as the first batch is added to the job.
func TestExecuteContext_MonitoringStartsAfterFirstBatch(t *testing.T) {
	fake := newFakeSession()

	// Track when GetJobInfo is called
	getJobInfoCalled := make(chan struct{}, 10)
	fake.customGetJobInfo = func(jobID string) (force.JobInfo, error) {
		select {
		case getJobInfoCalled <- struct{}{}:
		default:
		}

		// Simulate that all batches are complete
		return force.JobInfo{
			Id:                     jobID,
			ContentType:            "JSON",
			State:                  "Closed",
			NumberBatchesTotal:     2,
			NumberBatchesCompleted: 2,
			NumberBatchesFailed:    0,
			NumberRecordsProcessed: 6,
			NumberRecordsFailed:    0,
		}, nil
	}

	// recordSender sends multiple records with delays
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		// Send first batch worth
		for i := 0; i < 3; i++ {
			out <- force.ForceRecord{"Id": fmt.Sprintf("%d", i)}
		}
		// Delay before sending more records
		time.Sleep(150 * time.Millisecond)
		// Send second batch worth
		for i := 3; i < 6; i++ {
			out <- force.ForceRecord{"Id": fmt.Sprintf("%d", i)}
		}
		close(out)
		return nil
	}

	converter := func(r force.ForceRecord) []force.ForceRecord {
		return []force.ForceRecord{r}
	}

	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    3, // 3 records per batch
	}

	// Allow close to proceed after a delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		close(fake.closeAllow)
	}()

	// run ExecuteContext
	_, err := e.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext failed: %v", err)
	}

	// Verify that GetJobInfo was called (monitoring was active)
	select {
	case <-getJobInfoCalled:
		// Good, monitoring was active
	default:
		t.Fatal("GetJobInfo was not called - monitoring did not start")
	}
}

// TestExecuteContext_ContextCancellationDuringMonitoring ensures that context cancellation
// during monitoring is handled properly.
func TestExecuteContext_ContextCancellationDuringMonitoring(t *testing.T) {
	fake := newFakeSession()

	// Track GetJobInfo calls
	getJobInfoCount := 0
	mu := sync.Mutex{}
	fake.customGetJobInfo = func(jobID string) (force.JobInfo, error) {
		mu.Lock()
		getJobInfoCount++
		mu.Unlock()

		// Simulate delay in GetJobInfo
		time.Sleep(10 * time.Millisecond)

		// Return completed job
		return force.JobInfo{
			Id:                     jobID,
			ContentType:            "JSON",
			State:                  "Closed",
			NumberBatchesTotal:     1,
			NumberBatchesCompleted: 1,
			NumberBatchesFailed:    0,
			NumberRecordsProcessed: 1,
			NumberRecordsFailed:    0,
		}, nil
	}

	// recordSender sends one batch then closes
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		out <- force.ForceRecord{"Id": "1"}
		close(out)
		return nil
	}

	converter := func(r force.ForceRecord) []force.ForceRecord {
		return []force.ForceRecord{r}
	}

	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var err error

	go func() {
		_, err = e.ExecuteContext(ctx)
		close(done)
	}()

	// Wait for job to be closed
	select {
	case <-fake.closedInvoked:
		// Good
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CloseBulkJobWithContext was not called")
	}

	// Allow close to complete
	close(fake.closeAllow)

	// Let monitoring run for a bit
	time.Sleep(50 * time.Millisecond)

	// Cancel context during monitoring
	cancel()

	// Wait for ExecuteContext to complete
	select {
	case <-done:
		// Good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ExecuteContext did not return after context cancellation")
	}

	// Should have context error
	if err != nil && err != context.Canceled {
		t.Fatalf("expected context.Canceled or nil error, got %v", err)
	}

	// Should have made at least one GetJobInfo call
	mu.Lock()
	finalCount := getJobInfoCount
	mu.Unlock()

	if finalCount < 1 {
		t.Fatalf("expected at least 1 GetJobInfo call during monitoring, got %d", finalCount)
	}
}

// TestExecuteContext_NoBatchesAdded ensures that the execution handles the case
// where no batches are added (empty input).
func TestExecuteContext_NoBatchesAdded(t *testing.T) {
	fake := newFakeSession()

	// recordSender closes immediately without sending records
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		close(out)
		return nil
	}

	converter := func(r force.ForceRecord) []force.ForceRecord {
		return []force.ForceRecord{r}
	}

	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Converter:    converter,
		BatchSize:    1,
	}

	// Allow close to proceed immediately
	close(fake.closeAllow)

	// run ExecuteContext
	res, err := e.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext failed: %v", err)
	}

	// Should not have closed a job (no job was created)
	select {
	case <-fake.closedInvoked:
		t.Fatal("CloseBulkJobWithContext should not be called when no records are sent")
	default:
		// Good
	}

	// GetJobInfo should not have been called
	if fake.getJobInfoCallCount > 0 {
		t.Fatalf("GetJobInfo should not be called when no job is created, but was called %d times", fake.getJobInfoCallCount)
	}

	// Result should indicate no failures
	if res.NumberBatchesFailed() != 0 {
		t.Errorf("unexpected NumberBatchesFailed: %d", res.NumberBatchesFailed())
	}
}

func TestExecuteContext_ExprConverterPanic(t *testing.T) {
	fake := newFakeSession()
	// sender will send two records then close
	sender := func(ctx context.Context, out chan<- force.ForceRecord) error {
		out <- force.ForceRecord{"Id": "1"}
		out <- force.ForceRecord{"Id": "2"}
		close(out)
		return nil
	}
	// converter causes panic on second record
	e := Execution{
		Session:      fake,
		RecordSender: sender,
		Expr:         `incr("key") > 1 ? record.NoSuchObject.NoSuchField : record`,
		BatchSize:    1,
	}
	// run ExecuteContext in background so we can unblock CloseBulkJob
	ctx := context.Background()
	done := make(chan struct{})
	var err error
	go func() {
		_, err = e.ExecuteContext(ctx)
		close(done)
	}()
	// wait for CloseBulkJob to be invoked
	select {
	case <-fake.closedInvoked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected bulk job to be closed after converter panic")
	}
	// allow CloseBulkJob to return
	close(fake.closeAllow)
	// wait for ExecuteContext to finish
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ExecuteContext did not return after CloseBulkJob")
	}
	if err == nil || !strings.Contains(err.Error(), "panic occurred") {
		t.Fatalf("expected panic error, got %v", err)
	}
}
