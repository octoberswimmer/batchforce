package batch

import (
	force "github.com/ForceCLI/force/lib"
)

func Serialize(j *force.JobInfo) {
	j.ConcurrencyMode = "Serial"
}
