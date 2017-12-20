batchforce
==========

A go library to make bulk updates in Salesforce using a SOQL query and the Bulk
API.

The active [force CLI](https://github.com/ForceCLI/force) login is used, so log
in using `force login` or set your active user using `force active -a
<username>` before running your application.

Example
=======

```go
package main

import (
	batch "github.com/octoberswimmer/batchforce"
	force "github.com/heroku/force/lib"
	"time"
)

var (
	fiftyYearsAgo  = time.Now().AddDate(-50, 0, 0)
	thirtyYearsAgo = time.Now().AddDate(-30, 0, 0)
)

func main() {
	query := `
		SELECT
			Id,
			Birthdate
		FROM
			Contact
		WHERE
			Birthdate != null
	`

	batch.Run("Contact", query, setTitle)
}

func setTitle(record force.ForceRecord) (updates []force.ForceRecord) {
	birthdate, err := time.Parse("2006-01-02", record["Birthdate"].(string))
	if err != nil {
		return
	}
	update := force.ForceRecord{}
	update["Id"] = record["Id"].(string)
	switch {
	case birthdate.Before(fiftyYearsAgo):
		update["Title"] = "Geezer"
		updates = append(updates, update)
	case birthdate.After(thirtyYearsAgo):
		update["Title"] = "Whippersnapper"
		updates = append(updates, update)
	}
	return
}
```
