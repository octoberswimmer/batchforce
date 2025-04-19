package batch

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	force "github.com/ForceCLI/force/lib"
)

// fakeSession implements BulkSession for testing, blocking on CloseBulkJobWithContext until allowed.
type fakeSession struct {
	closedInvoked chan struct{}
	closeAllow    chan struct{}
}

func newFakeSession() *fakeSession {
	return &fakeSession{
		closedInvoked: make(chan struct{}, 1),
		closeAllow:    make(chan struct{}),
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
	return force.BatchInfo{Id: "batch-id"}, nil
}

func (f *fakeSession) CloseBulkJobWithContext(ctx context.Context, jobID string) (force.JobInfo, error) {
	// signal that CloseBulkJob was invoked
	f.closedInvoked <- struct{}{}
	// block until allowed
	<-f.closeAllow
	// return final job info with completed batch counts
	return force.JobInfo{
		Id:                     jobID,
		ContentType:            "JSON",
		NumberBatchesTotal:     1,
		NumberBatchesCompleted: 1,
		NumberBatchesFailed:    0,
	}, nil
}

func (f *fakeSession) GetJobInfo(jobID string) (force.JobInfo, error) {
	// always report job complete
	return force.JobInfo{
		Id:                     jobID,
		ContentType:            "JSON",
		NumberBatchesTotal:     1,
		NumberBatchesCompleted: 1,
		NumberBatchesFailed:    0,
	}, nil
}

func (f *fakeSession) GetBatches(jobID string) ([]force.BatchInfo, error) {
	return nil, nil
}

func (f *fakeSession) RetrieveBulkBatchResults(jobID, batchID string) (force.BatchResult, error) {
	return force.BatchResult{}, nil
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
