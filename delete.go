package batch

func Delete(j *BulkJob) {
	j.Operation = "delete"
}
