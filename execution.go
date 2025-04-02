package batch

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	force "github.com/ForceCLI/force/lib"
	"github.com/clbanning/mxj"
	"github.com/octoberswimmer/batchforce/soql"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Execution struct {
	Session *force.Force

	JobOptions []JobOption
	Object     string

	Apex string

	Expr      string
	Converter Converter

	RecordWriter RecordWriter

	Query        string
	QueryAll     bool
	CsvFile      string
	RecordSender RecordSender

	DryRun        bool
	PreviewWriter io.Writer
	BatchSize     int
}

type Result interface {
	NumberBatchesFailed() int
	NumberRecordsFailed() int
}

// Arbitrary function that can send ForceRecord's.
// A RecordSender should close the records channel when it's done writing.
type RecordSender func(ctx context.Context, records chan<- force.ForceRecord) error

// A RecordWriter can send write records somewhere other than a Salesforce Bulk API job
type RecordWriter func(ctx context.Context, records <-chan force.ForceRecord) (Result, error)

type Batch []force.ForceRecord
type Converter func(force.ForceRecord) []force.ForceRecord

type JobOption func(*force.JobInfo)

func NewExecution(object string, query string) *Execution {
	exec, err := NewQueryExecution(object, query)
	if err != nil {
		log.Fatalf("query error: %s", err.Error())
	}
	return exec
}

func NewQueryExecution(object string, query string) (*Execution, error) {
	if err := soql.Validate([]byte(query)); err != nil {
		return nil, err
	}

	return &Execution{
		Object:    object,
		Query:     query,
		BatchSize: 2000,
	}, nil
}

func NewCSVExecution(object string, csv string) (*Execution, error) {
	return &Execution{
		Object:    object,
		CsvFile:   csv,
		BatchSize: 2000,
	}, nil
}

func NewRecordSenderExecution(object string, sender RecordSender) (*Execution, error) {
	return &Execution{
		Object:       object,
		RecordSender: sender,
		BatchSize:    2000,
	}, nil
}

func (e *Execution) Execute() (Result, error) {
	return e.ExecuteContext(context.Background())
}

func (e *Execution) ExecuteContext(ctx context.Context) (Result, error) {
	var apexContext any
	var err error

	if e.Apex != "" {
		apexContext, err = e.getApexContext()
		if err != nil {
			return nil, fmt.Errorf("Unable to get apex context: %w", err)
		}
	}
	if e.Expr != "" {
		e.Converter, err = exprConverter(e.Expr, apexContext)
		if err != nil {
			return nil, fmt.Errorf("Expr error: %w", err)
		}
	} else if e.Converter == nil {
		return nil, fmt.Errorf("Expr or Converter must be defined")
	}

	converter := e.Converter
	if e.Query != "" {
		converter, err = makeFlatteningConverter(e.Query, converter)
		if err != nil {
			return nil, fmt.Errorf("Unable to get make converter: %w", err)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	resultChan := make(chan Result, 1) // Buffer size 1 so the sender doesn't block

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	session := e.session()
	// Start converter that takes input data, converts it to zero or more output
	// records and sends them to the bulk job
	g.Go(func() error {
		return processRecords(ctx, queried, updates, converter)
	})

	// Start job that sends records to the Bulk API
	g.Go(func() error {
		res, err := e.update(ctx, updates)
		if err != nil {
			log.Errorf("update failed: %s", err)
		}
		select {
		case resultChan <- res:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	// Get input
	g.Go(func() error {
		switch {
		case e.Query != "":
			var queryOptions []func(*force.QueryOptions)
			if e.QueryAll {
				queryOptions = append(queryOptions, func(options *force.QueryOptions) {
					options.QueryAll = true
				})
			}
			return session.CancelableQueryAndSend(ctx, e.Query, queried, queryOptions...)
		case e.CsvFile != "":
			f, err := os.Open(e.CsvFile)
			if err != nil {
				return fmt.Errorf("failed to open file: %s", err)
			}
			defer f.Close()
			return RecordsFromCsv(ctx, f, queried)
		case e.RecordSender != nil:
			return e.RecordSender(ctx, queried)
		default:
			return fmt.Errorf("No input defined")
		}
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	select {
	case res := <-resultChan:
		return res, nil
	default:
		return nil, fmt.Errorf("no result returned")
	}
}

func (e *Execution) JobResults(job *force.JobInfo) (force.BatchResult, error) {
	var results force.BatchResult
	batches, err := e.Session.GetBatches(job.Id)
	if err != nil {
		return results, fmt.Errorf("Failed to retrieve result batches: %w", err)
	}
	for _, b := range batches {
		result, err := e.Session.RetrieveBulkBatchResults(job.Id, b.Id)
		if err != nil {
			return force.BatchResult{}, fmt.Errorf("Failed to retrieve batch: %w", err)
		}
		results = append(results, result...)
	}

	return results, nil
}

func (e *Execution) RunContext(ctx context.Context) Result {
	job, err := e.ExecuteContext(ctx)
	if err != nil {
		log.Fatalln(err.Error())
	}
	return job
}

func (e *Execution) RunContextE(ctx context.Context) (Result, error) {
	job, err := e.ExecuteContext(ctx)
	return job, err
}

func (e *Execution) Run() Result {
	return e.RunContext(context.Background())
}

func (e *Execution) startJob() (force.JobInfo, error) {
	job := force.JobInfo{}
	job.Operation = "update"
	job.ContentType = "JSON"
	job.Object = e.Object

	for _, option := range e.JobOptions {
		option(&job)
	}

	jobInfo, err := e.Session.CreateBulkJob(job)
	if err != nil {
		return jobInfo, fmt.Errorf("Failed to create bulk job: %w", err)
	}
	return jobInfo, nil
}

func (e *Execution) update(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
	if e.DryRun {
		return e.dryRun(ctx, records)
	}
	if e.RecordWriter != nil {
		return e.RecordWriter(ctx, records)
	}
	return e.updateSalesforce(ctx, records)
}

type BulkJobResult struct {
	force.JobInfo
}

func (b BulkJobResult) NumberBatchesFailed() int {
	return b.JobInfo.NumberBatchesFailed
}

func (b BulkJobResult) NumberRecordsFailed() int {
	return b.JobInfo.NumberRecordsFailed
}

type dryRunResult struct {
}

func (b dryRunResult) NumberBatchesFailed() int {
	return 0
}

func (b dryRunResult) NumberRecordsFailed() int {
	return 0
}

func (e *Execution) dryRun(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
	defer func() {
		for range records {
			// drain records
		}
	}()
	previewWriter := e.PreviewWriter
	if previewWriter == nil {
		previewWriter = newlineWriter{w: os.Stdout}
	}

	waitForRecord := 5 * time.Second
	recordTimer := time.NewTimer(waitForRecord)
	defer recordTimer.Stop()

RECORDS:
	for {
		recordTimer.Reset(waitForRecord)
		select {
		case <-ctx.Done():
			log.Warn("Context canceled. Not sending more records to bulk job.")
			break RECORDS
		case record, ok := <-records:
			if !ok {
				break RECORDS
			}
			j, err := json.Marshal(record)
			if err != nil {
				log.Warnf("Invalid update: %s", err.Error())
			} else {
				fmt.Fprint(previewWriter, string(j))
			}
			continue
		case <-recordTimer.C:
			log.Info("Waiting for record to add to batch")
		}
	}
	return dryRunResult{}, nil
}

func (e *Execution) updateSalesforce(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
	defer func() {
		for range records {
			// drain records
		}
	}()
	var job force.JobInfo
	var err error

	batch := make(Batch, 0, e.BatchSize)

	sendBatch := func() error {
		updates, err := batch.marshallForBulkJob(job)
		if err != nil {
			return fmt.Errorf("Failed to serialize batch: %w", err)
		}
		log.Infof("Adding batch of %d records to job %s", len(batch), job.Id)
		_, err = e.Session.AddBatchToJob(string(updates), job)
		if err != nil {
			return fmt.Errorf("Failed to enqueue batch: %w", err)
		}
		batch = make(Batch, 0, e.BatchSize)
		return nil
	}
	waitForRecord := 5 * time.Second
	recordTimer := time.NewTimer(waitForRecord)
	defer recordTimer.Stop()

RECORDS:
	for {
		recordTimer.Reset(waitForRecord)
		select {
		case <-ctx.Done():
			log.Warn("Context canceled. Not sending more records to bulk job.")
			break RECORDS
		case record, ok := <-records:
			if !ok {
				break RECORDS
			}
			if job.Id == "" {
				job, err = e.startJob()
				if err != nil {
					return BulkJobResult{JobInfo: job}, err
				}
				log.Infof("Created job %s", job.Id)
			}
			batch = append(batch, record)
			if len(batch) == e.BatchSize {
				err := sendBatch()
				if err != nil {
					log.Error(err.Error())
					break RECORDS
				}
			}
		case <-recordTimer.C:
			log.Info("Waiting for record to add to batch")
		}
	}
	if len(batch) > 0 {
		err := sendBatch()
		if err != nil {
			log.Error(err.Error())
		}
	}
	if job.Id == "" {
		log.Info("Bulk job not started")
		return BulkJobResult{JobInfo: job}, err
	}
	log.Info("Closing bulk job")
	job, err = e.Session.CloseBulkJobWithContext(ctx, job.Id)
	if err != nil {
		return BulkJobResult{JobInfo: job}, fmt.Errorf("Failed to close bulk job: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return BulkJobResult{JobInfo: job}, fmt.Errorf("Cancelling wait for bulk job completion: %w", ctx.Err())
		default:
		}
		job, err = e.Session.GetJobInfo(job.Id)
		if err != nil {
			return BulkJobResult{JobInfo: job}, fmt.Errorf("Failed to get bulk job status: %w", err)
		}
		log.Info(fmt.Sprintf("Records Processed: %d | Records Failed: %d", job.NumberRecordsProcessed, job.NumberRecordsFailed))
		log.Info(fmt.Sprintf("Batches In Progress: %d | Batches Complete: %d/%d", job.NumberBatchesInProgress, job.NumberBatchesCompleted, job.NumberBatchesTotal))
		if job.State == "Aborted" || job.State == "Failed" {
			return BulkJobResult{JobInfo: job}, fmt.Errorf("Bulk Job %s", job.State)
		}
		if job.NumberBatchesCompleted+job.NumberBatchesFailed == job.NumberBatchesTotal {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	return BulkJobResult{JobInfo: job}, nil
}

func processRecords(ctx context.Context, input <-chan force.ForceRecord, output chan<- force.ForceRecord, converter Converter) (err error) {
	defer func() {
		close(output)
		for range input {
			// drain records
		}
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %s", r)
		}
	}()
	waitForRecord := 1 * time.Second
	recordTimer := time.NewTimer(waitForRecord)
	defer recordTimer.Stop()

INPUT:
	for {
		recordTimer.Reset(waitForRecord)
		select {
		case record, more := <-input:
			if !more {
				log.Info("Done processing input records")
				break INPUT
			}
			updates := converter(record)
			for _, update := range updates {
				select {
				case <-ctx.Done():
				case output <- update:
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("Processing canceled: %w", ctx.Err())
		case <-recordTimer.C:
			log.Info("Waiting for record to convert")
		}
	}
	return nil
}

func (e *Execution) session() *force.Force {
	if e.Session == nil {
		session, err := force.ActiveForce()
		if err != nil {
			log.Fatalf("Failed to get active force session: %s", err.Error())
		}
		e.Session = session
	}
	return e.Session
}

func (batch Batch) marshallForBulkJob(job force.JobInfo) (updates []byte, err error) {
	switch strings.ToUpper(job.ContentType) {
	case "JSON":
		updates, err = json.Marshal(batch)
	case "XML":
		xmlData := new(bytes.Buffer)

		xmlData.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<sObjects xmlns="http://www.force.com/2009/06/asyncapi/dataload">`))
		for _, record := range batch {
			mv := mxj.Map(record)
			err = mv.XmlIndentWriter(xmlData, "", "  ", "sObject")
			if err != nil {
				return
			}
		}
		xmlData.Write([]byte(`</sObjects>`))
		updates = xmlData.Bytes()
	case "CSV":
		return batchToCsv(batch)
	default:
		err = fmt.Errorf("Unsupported ContentType: " + job.ContentType)
	}
	return
}

func batchToCsv(batch Batch) ([]byte, error) {
	csvData := new(bytes.Buffer)
	if len(batch) == 0 {
		return csvData.Bytes(), nil
	}
	csvWriter := csv.NewWriter(csvData)
	header := make([]string, 0, len(batch[0]))
	for key := range batch[0] {
		header = append(header, key)
	}
	err := csvWriter.Write(header)
	if err != nil {
		return csvData.Bytes(), err
	}
	for _, record := range batch {
		row := make([]string, len(record))
		for i, key := range header {
			switch v := record[key].(type) {
			case string:
				row[i] = v
			case int64:
				row[i] = strconv.FormatInt(v, 10)
			default:
				panic(fmt.Sprintf("%+v is %T", v, v))
			}
		}
		err := csvWriter.Write(row)
		if err != nil {
			return csvData.Bytes(), err
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return csvData.Bytes(), err
	}
	return csvData.Bytes(), nil
}

type newlineWriter struct {
	w io.Writer
}

func (nw newlineWriter) Write(p []byte) (int, error) {
	n, err := nw.w.Write(p)
	if err != nil {
		return n, err
	}
	_, err = nw.w.Write([]byte("\n"))
	return n, err
}
