package batch

import (
	"encoding/json"
	"fmt"
	force "github.com/heroku/force/lib"
	"os"
	"time"
)

type BulkJob struct {
	force.JobInfo
	BatchSize int
}

func update(session *force.Force, records <-chan force.ForceRecord, failures chan<- int, jobOptions ...func(*BulkJob)) {
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
	batch := make([]force.ForceRecord, 0, job.BatchSize)

	sendBatch := func() {
		updates, err := json.Marshal(batch)
		if err != nil {
			fmt.Println("Failed to serialize batch: " + err.Error())
			os.Exit(1)
		}
		fmt.Printf("Adding batch of %d records to job %s\n", len(batch), jobInfo.Id)
		_, err = session.AddBatchToJob(string(updates), jobInfo)
		if err != nil {
			fmt.Println("Failed to enqueue batch: " + err.Error())
		}
		batch = make([]force.ForceRecord, 0, job.BatchSize)
	}

	for record := range records {
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
		force.DisplayJobInfo(status)
		if status.NumberBatchesCompleted+status.NumberBatchesFailed == status.NumberBatchesTotal {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	failures <- status.NumberRecordsFailed
}

func processRecords(input <-chan force.ForceRecord, output chan<- force.ForceRecord, converter func(force.ForceRecord) []force.ForceRecord) {
	for record := range input {
		updates := converter(record)
		for _, update := range updates {
			output <- update
		}
	}
	close(output)
}

func Run(sobject string, query string, converter func(force.ForceRecord) []force.ForceRecord, jobOptions ...func(*BulkJob)) int {
	session, err := force.ActiveForce()
	if err != nil {
		os.Exit(1)
	}

	setObject := func(job *BulkJob) {
		job.Object = sobject
	}
	jobOptions = append([]func(*BulkJob){setObject}, jobOptions...)

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	numberFailers := make(chan int)
	go processRecords(queried, updates, converter)
	go update(session, updates, numberFailers, jobOptions...)
	err = session.QueryAndSend(query, queried)
	if err != nil {
		fmt.Println("Query failed: " + err.Error())
		os.Exit(1)
	}
	return <-numberFailers
}
