## batchforce publish

publish Platform Events using the Pub/Sub API

```
batchforce publish [flags] /event/<Event> <Expr>
```

### Examples

```

$ batchforce publish --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" /event/Account_Change__e '{Account__c: record.Id, Name: record.Name + " Copy"}'
$ batchforce publish --file accounts.csv /event/Account_Change__e '{Id: record.Id, Name: record.Name + " Copy"}'
	
```

### Options

```
  -c, --context string     provide context with anonymous apex
  -n, --dry-run            dry run.  Display updates without modifying records
  -f, --file string        CSV file for input data
  -h, --help               help for publish
  -q, --query string       SOQL query for input data
      --query-all string   query all records (including archived/deleted records)
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
      --help-expr          show expr language definition
      --quiet              suppress informational log messages
```

### SEE ALSO

* [batchforce](batchforce.md)	 - Use Bulk API to update Salesforce records

