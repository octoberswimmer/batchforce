package batch

import force "github.com/ForceCLI/force/lib"

func Delete(j *force.JobInfo) {
	j.Operation = "delete"
}

func HardDelete(j *force.JobInfo) {
	j.Operation = "hardDelete"
}
