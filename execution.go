package batch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	force "github.com/ForceCLI/force/lib"
	"github.com/clbanning/mxj"
	"github.com/octoberswimmer/batchforce/soql"
	log "github.com/sirupsen/logrus"
)

type Execution struct {
	Session    *force.Force
	JobOptions []JobOption
	Object     string
	Apex       string
	Expr       string
	Converter  Converter

	Query        string
	CsvFile      string
	RecordSender RecordSender

	DryRun    bool
	BatchSize int
}

// Arbitrary function that can send ForceRecord's and stop sending on abort.RecordSender
// A RecordSender should close the records channel when it's done writing.
type RecordSender func(records chan<- force.ForceRecord, abort <-chan bool) error

type ProcessorChannels struct {
	input  <-chan force.ForceRecord
	output chan<- force.ForceRecord
	abort  chan<- bool
}

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

func (e *Execution) Execute() (force.JobInfo, error) {
	var context any
	var err error
	var result force.JobInfo
	if e.Apex != "" {
		context, err = e.getApexContext()
		if err != nil {
			return result, fmt.Errorf("Unable to get apex context: %w", err)
		}
	}
	if e.Expr != "" {
		e.Converter = exprConverter(e.Expr, context)
	} else if e.Converter == nil {
		return result, fmt.Errorf("Expr or Converter must be defined")
	}

	session := e.session()

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	abortChannel := make(chan bool)
	jobResult := make(chan force.JobInfo)
	go e.update(updates, jobResult)
	switch {
	case e.Query != "":
		go processRecordsWithSubQueries(e.Query, ProcessorChannels{input: queried, output: updates, abort: abortChannel}, e.Converter)
		err = session.AbortableQueryAndSend(e.Query, queried, abortChannel)
		if err != nil {
			return result, fmt.Errorf("Query failed: %w", err)
		}
	case e.CsvFile != "":
		go processRecords(ProcessorChannels{input: queried, output: updates, abort: abortChannel}, e.Converter)
		err = recordsFromCsv(e.CsvFile, queried, abortChannel)
		if err != nil {
			return result, fmt.Errorf("Failed to read file: %w", err)
		}
	case e.RecordSender != nil:
		go processRecords(ProcessorChannels{input: queried, output: updates, abort: abortChannel}, e.Converter)
		err = e.RecordSender(queried, abortChannel)
		if err != nil {
			return result, fmt.Errorf("Query failed: %w", err)
		}
	default:
		return result, fmt.Errorf("No input defined")
	}
	select {
	case result = <-jobResult:
		return result, nil
	case <-abortChannel:
		return result, fmt.Errorf("Aborted")
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
			return force.BatchResult{}, fmt.Errorf("Failed to retrieve batche: %w", err)
		}
		results = append(results, result...)
	}

	return results, nil
}

func (e *Execution) Run() force.JobInfo {
	job, err := e.Execute()
	if err != nil {
		log.Fatalln(err.Error())
	}
	return job
}

func (e *Execution) update(records <-chan force.ForceRecord, result chan<- force.JobInfo) {
	job := force.JobInfo{}
	job.Operation = "update"
	job.ContentType = "JSON"
	job.Object = e.Object

	for _, option := range e.JobOptions {
		option(&job)
	}

	jobInfo, err := e.Session.CreateBulkJob(job)
	if err != nil {
		log.Fatalln("Failed to create bulk job: " + err.Error())
	}
	log.Infof("Created job %s\n", jobInfo.Id)
	batch := make(Batch, 0, e.BatchSize)

	sendBatch := func() {
		updates, err := batch.marshallForBulkJob(job)
		if err != nil {
			log.Fatalln("Failed to serialize batch: " + err.Error())
		}
		log.Infof("Adding batch of %d records to job %s\n", len(batch), jobInfo.Id)
		_, err = e.Session.AddBatchToJob(string(updates), jobInfo)
		if err != nil {
			log.Fatalln("Failed to enqueue batch: " + err.Error())
		}
		batch = make(Batch, 0, e.BatchSize)
	}

	for record := range records {
		if e.DryRun {
			j, err := json.Marshal(record)
			if err != nil {
				log.Warnf("Invalid update: %s", err.Error())
			} else {
				fmt.Println(string(j))
			}
			continue
		}
		batch = append(batch, record)
		if len(batch) == e.BatchSize {
			sendBatch()
		}
	}
	if len(batch) > 0 {
		sendBatch()
	}
	jobInfo, err = e.Session.CloseBulkJob(jobInfo.Id)
	if err != nil {
		log.Fatalln("Failed to close bulk job: " + err.Error())
	}
	var status force.JobInfo
	for {
		status, err = e.Session.GetJobInfo(jobInfo.Id)
		if err != nil {
			log.Fatalln("Failed to get bulk job status: " + err.Error())
		}
		force.DisplayJobInfo(status, os.Stderr)
		if status.NumberBatchesCompleted+status.NumberBatchesFailed == status.NumberBatchesTotal {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	result <- status
}

func processRecords(channels ProcessorChannels, converter Converter) {
	defer func() {
		close(channels.output)
		if err := recover(); err != nil {
			select {
			// Make sure sender isn't blocked waiting for us to read
			case <-channels.input:
			default:
			}
			log.Errorln("panic occurred:", err)
			log.Errorln("Sending abort signal")
			channels.abort <- true
		}
	}()

	for record := range channels.input {
		updates := converter(record)
		for _, update := range updates {
			channels.output <- update
		}
	}
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
	default:
		err = fmt.Errorf("Unsupported ContentType: " + job.ContentType)
	}
	return
}
