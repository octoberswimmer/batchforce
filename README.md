batchforce
==========

A go library and CLI to make bulk updates in Salesforce using a SOQL query and the Bulk
API.

The active [force CLI](https://github.com/ForceCLI/force) login is used, so log
in using `force login` or set your active user using `force active -a
<username>` before running your application.

CLI Example
===========

Each record is made available to [expr](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md) as
`record`.  The expr expression should evaluate to a single map or an array of
maps.


```
$ batchforce update Account "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%Test'" '{Id: record.Id, Name: record.Name + " Test"}'
```

This will query all Accounts whose Name doesn't end with "Test" and append "Test" to the Name.

CLI Help
========

```
$ batchforce help

        Insert/Update Salesforce records using the Bulk API and a SOQL query.

        Optionally use anonymous apex to provide additional context.

        The SOQL query is used to generate the input.  Each record returned by the
        query is made available to the Expr expression as a map named "record".  See
        https://expr.medv.io/ for details on the Expr language.  The expression
        evaluate to an map of the form, "{ Field: Value, ... }".

        In addition to Expr's built-in operators and functions, the following
        functions can be used within the expression:
        - stripHtml: removes HTML tags
        - escapeHtml: escapes characters using HTML entities like Apex's
          String.escapeHtml4 method
        - base64: base-64 encodes input


        Additional context to be provided to the Expr expression by passing the
        --context parameter containining anonymous apex to execute before the
        records are queried.  Each apex variable defined will be available within
        the "apex" map.

Usage:
  batchforce
  batchforce [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  insert      insert Salesforce records using the Bulk API
  update      Update Salesforce records using the Bulk API

Flags:
  -h, --help   help for batchforce

Use "batchforce [command] --help" for more information about a command.
```

Library Example
===============

```go
package main

import (
	batch "github.com/octoberswimmer/batchforce"
	force "github.com/ForceCLI/force/lib"
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
