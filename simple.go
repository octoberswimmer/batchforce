package batch

import (
	force "github.com/ForceCLI/force/lib"
	log "github.com/sirupsen/logrus"
)

func Run(sobject string, query string, converter Converter, jobOptions ...JobOption) Result {
	e := NewExecution(sobject, query)

	session, err := force.ActiveForce()
	if err != nil {
		log.Fatalf("Failed to get active force session: %s", err.Error())
	}
	e.Session = session

	e.JobOptions = jobOptions
	e.Converter = converter
	return e.Run()
}
