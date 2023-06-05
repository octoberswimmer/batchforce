package batch

import force "github.com/ForceCLI/force/lib"

func Insert(j *force.JobInfo) {
	j.Operation = "insert"
}
