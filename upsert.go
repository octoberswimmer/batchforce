package batch

import force "github.com/ForceCLI/force/lib"

func ExternalId(e string) JobOption {
	return func(j *force.JobInfo) {
		j.ExternalIdFieldName = e
	}
}

func Upsert(j *force.JobInfo) {
	j.Operation = "upsert"
}
