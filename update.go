package batch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	anon "github.com/octoberswimmer/batchforce/apex"
	"github.com/octoberswimmer/batchforce/soql"

	force "github.com/ForceCLI/force/lib"
	"github.com/antonmedv/expr"
	"github.com/benhoyt/goawk/interp"
	"github.com/clbanning/mxj"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
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
		log.Fatalln("Failed to create bulk job: " + err.Error())
	}
	log.Infof("Created job %s\n", jobInfo.Id)
	batch := make(Batch, 0, job.BatchSize)

	sendBatch := func() {
		updates, err := batch.marshallForBulkJob(job)
		if err != nil {
			log.Fatalln("Failed to serialize batch: " + err.Error())
		}
		log.Infof("Adding batch of %d records to job %s\n", len(batch), jobInfo.Id)
		_, err = session.AddBatchToJob(string(updates), jobInfo)
		if err != nil {
			log.Fatalln("Failed to enqueue batch: " + err.Error())
		}
		batch = make(Batch, 0, job.BatchSize)
	}

	for record := range records {
		if job.dryRun {
			j, err := json.Marshal(record)
			if err != nil {
				log.Warnf("Invalid update: %s", err.Error())
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
		log.Fatalln("Failed to close bulk job: " + err.Error())
	}
	var status force.JobInfo
	for {
		status, err = session.GetJobInfo(jobInfo.Id)
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

func processRecords(query string, channels ProcessorChannels, converter func(force.ForceRecord) []force.ForceRecord) {
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

	subQueryRelationships, err := soql.SubQueryRelationships(query)
	if err != nil {
		panic("Failed to parse query for subqueries: " + err.Error())
	}

	for record := range channels.input {
		updates := converter(flattenRecord(record, subQueryRelationships))
		for _, update := range updates {
			channels.output <- update
		}
	}
}

// Replace subquery results with the records for the sub-query
func flattenRecord(r force.ForceRecord, subQueryRelationships map[string]bool) force.ForceRecord {
	if len(subQueryRelationships) == 0 {
		return r
	}
	for k, v := range r {
		if v == nil {
			continue
		}
		if _, found := subQueryRelationships[strings.ToLower(k)]; found {
			subQuery := v.(map[string]interface{})
			records := subQuery["records"].([]interface{})
			done := subQuery["done"].(bool)
			if !done {
				log.Fatalln("got possible incomplete reesults for " + k + " subquery. done is false, but all results should have been retrieved")
			}
			r[k] = records
		}
	}
	return r
}

func RunExpr(sobject string, query string, expression string, jobOptions ...JobOption) force.JobInfo {
	return RunExprWithApex(sobject, query, expression, "", jobOptions...)
}

func RunExprWithApex(sobject string, query string, expression string, apex string, jobOptions ...JobOption) force.JobInfo {
	var context any
	var err error
	if apex != "" {
		context, err = getApexContext(apex)
		if err != nil {
			log.Fatalln("Unable to get apex context:", err)
		}
	}
	env := map[string]interface{}{
		"record": force.ForceRecord{},
		"apex":   context,
	}
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		log.Fatalln("Invalid expression:", err)
	}
	converter := func(record force.ForceRecord) []force.ForceRecord {
		env := map[string]interface{}{
			"record": record,
			"apex":   context,
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
		log.Warnln("Unexpected value.  It should be a map or array or maps.  Got", out)
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
	if err := soql.Validate([]byte(query)); err != nil {
		log.Fatalf("query error: %s", err.Error())
	}

	session, err := force.ActiveForce()
	if err != nil {
		log.Fatalf("Failed to get active force session: %s", err.Error())
	}

	setObject := func(job *BulkJob) {
		job.Object = sobject
	}
	jobOptions = append([]JobOption{setObject}, jobOptions...)

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	abortChannel := make(chan bool)
	jobResult := make(chan force.JobInfo)
	go processRecords(query, ProcessorChannels{input: queried, output: updates, abort: abortChannel}, converter)
	go update(session, updates, jobResult, jobOptions...)
	err = session.AbortableQueryAndSend(query, queried, abortChannel)
	if err != nil {
		log.Fatalln("Query failed: " + err.Error())
	}
	select {
	case r := <-jobResult:
		return r
	case <-abortChannel:
		log.Fatalln("Aborted")
		return force.JobInfo{}
	}
}

func getApexContext(apex string) (map[string]any, error) {
	apexVars, err := anon.Vars(apex)
	if err != nil {
		return nil, err
	}
	lines := []string{"\n" + `Map<String, Object> b_f_c_t_x = new Map<String, Object>();`}
	for _, v := range apexVars {
		lines = append(lines, fmt.Sprintf(`b_f_c_t_x.put('%s', %s);`, v, v))
	}
	lines = append(lines, `System.debug(JSON.serialize(b_f_c_t_x));`)
	apex = apex + strings.Join(lines, "\n")
	session, err := force.ActiveForce()
	if err != nil {
		return nil, err
	}
	debugLog, err := session.Partner.ExecuteAnonymous(apex)
	if err != nil {
		return nil, err
	}
	val, err := varFromDebugLog(debugLog)
	if err != nil {
		return nil, err
	}
	var n map[string]any
	err = json.Unmarshal(val, &n)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func varFromDebugLog(log string) ([]byte, error) {
	input := strings.NewReader(log)
	output := new(bytes.Buffer)
	err := interp.Exec(`$2~/USER_DEBUG/ { var = $5 } END { print var }`, "|", input, output)
	if err != nil {
		return nil, err
	}
	return output.Bytes(), nil
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

func Serialize(j *BulkJob) {
	j.ConcurrencyMode = "Serial"
}

func BatchSize(n int) func(*BulkJob) {
	return func(j *BulkJob) {
		j.BatchSize = n
	}
}
