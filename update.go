package batch

import (
	"bytes"
	"encoding/json"
	"fmt"

	force "github.com/ForceCLI/force/lib"
	"github.com/antonmedv/expr"
	"github.com/clbanning/mxj"
	"github.com/mitchellh/mapstructure"

	"os"
	"strings"
	"time"
)

type BulkJob struct {
	force.JobInfo
	BatchSize int
	dryRun    bool
}

type Batch []force.ForceRecord

type JobOption func(*BulkJob)

type BatchSession struct {
	Force *force.Force
}

func (batch Batch) marshallForBulkJob(job BulkJob) (updates []byte, err error) {
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

func update(session *force.Force, records <-chan force.ForceRecord, result chan<- force.JobInfo, jobOptions ...JobOption) {
	job := BulkJob{
		BatchSize: 2000,
	}
	job.Operation = "update"
	job.ContentType = "JSON"

	for _, option := range jobOptions {
		option(&job)
	}

	jobInfo, err := session.CreateBulkJob(job.JobInfo)
	if err != nil {
		fmt.Println("Failed to create bulk job: " + err.Error())
		os.Exit(1)
	}
	fmt.Printf("Created job %s\n", jobInfo.Id)
	batch := make(Batch, 0, job.BatchSize)

	sendBatch := func() {
		updates, err := batch.marshallForBulkJob(job)
		if err != nil {
			fmt.Println("Failed to serialize batch: " + err.Error())
			os.Exit(1)
		}
		fmt.Printf("Adding batch of %d records to job %s\n", len(batch), jobInfo.Id)
		_, err = session.AddBatchToJob(string(updates), jobInfo)
		if err != nil {
			fmt.Println("Failed to enqueue batch: " + err.Error())
			os.Exit(1)
		}
		batch = make(Batch, 0, job.BatchSize)
	}

	for record := range records {
		if job.dryRun {
			j, err := json.Marshal(record)
			if err != nil {
				fmt.Printf("Invalid update: %s", err.Error())
			} else {
				fmt.Println(string(j))
			}
			continue
		}
		batch = append(batch, record)
		if len(batch) == job.BatchSize {
			sendBatch()
		}
	}
	if len(batch) > 0 {
		sendBatch()
	}
	jobInfo, err = session.CloseBulkJob(jobInfo.Id)
	if err != nil {
		fmt.Println("Failed to close bulk job: " + err.Error())
		os.Exit(1)
	}
	var status force.JobInfo
	for {
		status, err = session.GetJobInfo(jobInfo.Id)
		if err != nil {
			fmt.Println("Failed to get bulk job status: " + err.Error())
			os.Exit(1)
		}
		force.DisplayJobInfo(status, os.Stdout)
		if status.NumberBatchesCompleted+status.NumberBatchesFailed == status.NumberBatchesTotal {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	result <- status
}

func processRecords(channels ProcessorChannels, converter func(force.ForceRecord) []force.ForceRecord) {
	defer func() {
		close(channels.output)
		if err := recover(); err != nil {
			select {
			// Make sure sender isn't blocked waiting for us to read
			case <-channels.input:
			default:
			}
			fmt.Println("panic occurred:", err)
			fmt.Println("Sending abort signal")
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

func RunExpr(sobject string, query string, expression string, jobOptions ...JobOption) force.JobInfo {
	env := map[string]interface{}{
		"record": force.ForceRecord{},
	}
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		fmt.Println("Invalid expression:", err)
		os.Exit(1)
	}
	converter := func(record force.ForceRecord) []force.ForceRecord {
		env := map[string]interface{}{
			"record": record,
		}
		out, err := expr.Run(program, env)
		if err != nil {
			panic(err)
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
		fmt.Println("Unexpected value.  It should be a map or array or maps.  Got", out)
		return []force.ForceRecord{}
	}
	return Run(sobject, query, converter, jobOptions...)
}

type ProcessorChannels struct {
	input  <-chan force.ForceRecord
	output chan<- force.ForceRecord
	abort  chan<- bool
}

func Run(sobject string, query string, converter func(force.ForceRecord) []force.ForceRecord, jobOptions ...JobOption) force.JobInfo {
	session, err := force.ActiveForce()
	if err != nil {
		os.Exit(1)
	}

	setObject := func(job *BulkJob) {
		job.Object = sobject
	}
	jobOptions = append([]JobOption{setObject}, jobOptions...)

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	abortChannel := make(chan bool)
	jobResult := make(chan force.JobInfo)
	go processRecords(ProcessorChannels{input: queried, output: updates, abort: abortChannel}, converter)
	go update(session, updates, jobResult, jobOptions...)
	err = session.AbortableQueryAndSend(query, queried, abortChannel)
	if err != nil {
		fmt.Println("Query failed: " + err.Error())
		os.Exit(1)
	}
	select {
	case r := <-jobResult:
		return r
	case <-abortChannel:
		fmt.Println("Aborted")
		os.Exit(1)
		return force.JobInfo{}
	}
}

func (session *BatchSession) Load(sobject string, input <-chan force.ForceRecord, jobOptions ...JobOption) force.JobInfo {
	setObject := func(job *BulkJob) {
		job.Object = sobject
	}
	jobOptions = append([]JobOption{setObject}, jobOptions...)

	jobResult := make(chan force.JobInfo)
	go update(session.Force, input, jobResult, jobOptions...)
	return <-jobResult
}

func DryRun(j *BulkJob) {
	j.dryRun = true
}
