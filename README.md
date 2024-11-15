# batchforce

A command-line application to make bulk updates in Salesforce using the Bulk API.

* Use a SOQL query to retrieve the input data.
* Use an expr expression to transform the input into the data to load into Salesforce

The active [force CLI](https://github.com/ForceCLI/force) login is used, so log
in using `force login` or set your active user using `force active -a
<username>` before running your application.

## Installation

Download the latest release from
https://github.com/octoberswimmer/batchforce/releases.  Executables are
available for Windows, Linux and MacOS.

Install from source:
```
$ go install github.com/octoberswimmer/batchforce/cmd/batchforce@latest
```

## Web Version

There's also a web-based version at https://batchforce.octoberswimmer.com/,
which uses Salesforce's APIs directly from your browser. (*After authenticating and approving the application, a [CorsWhitelistOrigin](https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_corswhitelistorigin.htm) component is automatically deployed to your org to allow the batchforce application running in your browser to access your org.*)

## (Optional) Context

The `--context` flag can be used to provide additional org-specific contextual
data which can be referenced in the expression through an `apex` object.  It
takes a string containing anonymous apex.  Any variables defined will be made
available as keys within `apex`.

For example, if you have a "Supplier" Account Record Type, you could get the
Record Type's Id in a batchforce job to update the Record Type of Accounts from
"Partner" to "Supplier".

```
$ batchforce update Account -q "SELECT Id FROM Account WHERE RecordType.DeveloperName = 'Partner'" \
 '{Id: record.Id, RecordTypeId: apex.supplierId}' \
 --context "Id supplierId = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('Supplier').getRecordTypeId();"
```

## Example Usage

Each record is made available to [expr](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md) as
`record`.  The expr expression should evaluate to a single map or an array of
maps.


```
$ batchforce update Account --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%Test'" \
  '{Id: record.Id, Name: record.Name + " Test"}'
```

This will query all Accounts whose Name doesn't end with "Test" and append "Test" to the Name.

See [docs/batchforce.md](docs/batchforce.md) for all supported commands and the
[wiki](https://github.com/octoberswimmer/batchforce/wiki) for more examples.

## CLI Help

```
$ batchforce help

        Insert/Update/Delete Salesforce records using the Bulk API and a SOQL query.

        Optionally use anonymous apex to provide additional context.

        The SOQL query is used to generate the input.  Each record returned by the
        query is made available to the Expr expression as a map named "record".  See
        https://expr.medv.io/ for details on the Expr language.  The expression should
        evaluate to an map of the form, "{ Field: Value, ... }" or an array of such
        maps.

        In addition to Expr's built-in operators and functions, the following
        functions can be used within the expression:
        - stripHtml: removes HTML tags
        - escapeHtml: escapes characters using HTML entities like Apex's
          String.escapeHtml4 method
        - escapeUnicode: escapes characters using Unicode escape sequences
          like Apex's String.escapeUnicode method
        - base64: base-64 encodes input
        - md5: md5 hash of string
        - getSet: set key to value, returning previous value for key
        - compareAndSet: check if key maps to value; if key doesn't exist, set it to
          value (return true unless key already exists with different value)
        - changeValue: update value associated with key (returns true unless the key
          already exists and the value is unchanged)
        - incr: increments the number stored at key by one. set to 1 if not set.
        - clone: create a copy of the record

        The + and - operators can be used to add, update, or remove fields on the
        record object.  For example:
        record + {RecordTypeId: apex.myRecordTypeId} - "RecordType.Name"

        If creating multiple records from a source record, use clone to avoid mutating
        the same object repeatedly.  For example:
        1..100 | map(clone(record) + {Name: "Record " + string(#)})

        Additional context to be provided to the Expr expression by passing the
        --context parameter containining anonymous apex to execute before the
        records are queried.  Each apex variable defined will be available within
        the "apex" map.

        A csv file can be used as input instead of a SOQL query by using the --file
        parameter.  This is often useful when combined with --apex to map input to
        org-specific values such as Record Type Ids.

Usage:
  batchforce
  batchforce [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  delete      delete Salesforce records using the Bulk API
  help        Help about any command
  insert      insert Salesforce records using the Bulk API
  update      Update Salesforce records using the Bulk API
  upsert      upsert Salesforce records using the Bulk API
  version     Display current version

Flags:
  -a, --account username   account username to use
  -h, --help               help for batchforce
      --quiet              suppress informational log messages

Use "batchforce [command] --help" for more information about a command.
```
