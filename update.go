package batch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	anon "github.com/octoberswimmer/batchforce/apex"
	"github.com/octoberswimmer/batchforce/soql"
	csvmap "github.com/recursionpharma/go-csv-map"

	force "github.com/ForceCLI/force/lib"
	"github.com/antonmedv/expr"
	"github.com/benhoyt/goawk/interp"
	"github.com/clbanning/mxj"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

type Execution struct {
	Session    *force.Force
	JobOptions []JobOption
	Object     string
	Query      string
	CsvFile    string
	Apex       string
	Expr       string
	Converter  Converter

	DryRun    bool
	BatchSize int
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

func processRecordsWithSubQueries(query string, channels ProcessorChannels, converter Converter) {
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

func exprConverter(expression string, context any) func(force.ForceRecord) []force.ForceRecord {
	env := map[string]interface{}{
		"record": force.ForceRecord{},
		"apex":   context,
	}
	program, err := expr.Compile(expression, append(exprFunctions(), expr.Env(env))...)
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
		if out == nil {
			return []force.ForceRecord{}
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
	return converter
}

func (e *Execution) Run() force.JobInfo {
	var context any
	var err error
	if e.Apex != "" {
		context, err = e.getApexContext()
		if err != nil {
			log.Fatalln("Unable to get apex context:", err)
		}
	}
	if e.Expr != "" {
		e.Converter = exprConverter(e.Expr, context)
	} else if e.Converter == nil {
		log.Fatalf("Expr or Converter must be defined")
	}

	session := e.session()

	queried := make(chan force.ForceRecord)
	updates := make(chan force.ForceRecord)
	abortChannel := make(chan bool)
	jobResult := make(chan force.JobInfo)
	go e.update(updates, jobResult)
	if e.Query != "" {
		go processRecordsWithSubQueries(e.Query, ProcessorChannels{input: queried, output: updates, abort: abortChannel}, e.Converter)
		err = session.AbortableQueryAndSend(e.Query, queried, abortChannel)
		if err != nil {
			log.Fatalln("Query failed: " + err.Error())
		}
	} else {
		go processRecords(ProcessorChannels{input: queried, output: updates, abort: abortChannel}, e.Converter)
		err = recordsFromCsv(e.CsvFile, queried, abortChannel)
		if err != nil {
			log.Fatalln("Failed to read file: " + err.Error())
		}
	}
	select {
	case r := <-jobResult:
		return r
	case <-abortChannel:
		log.Fatalln("Aborted")
		return force.JobInfo{}
	}
}

func recordsFromCsv(fileName string, processor chan<- force.ForceRecord, abort <-chan bool) error {
	defer func() {
		close(processor)
	}()

	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	r := csvmap.NewReader(f)
	r.Columns, err = r.ReadHeader()
	if err != nil {
		return fmt.Errorf("failed to read csv header: %w", err)
	}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read csv row: %w", err)
		}
		select {
		case <-abort:
			break
		default:
			r := force.ForceRecord{}
			for k, v := range row {
				r[k] = any(v)
			}
			processor <- r
		}
	}

	return nil
}

type ProcessorChannels struct {
	input  <-chan force.ForceRecord
	output chan<- force.ForceRecord
	abort  chan<- bool
}

func (e *Execution) getApexContext() (map[string]any, error) {
	apex := e.Apex
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

	session := e.session()
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

func Serialize(j *force.JobInfo) {
	j.ConcurrencyMode = "Serial"
}
