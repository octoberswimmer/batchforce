## batchforce delete

delete Salesforce records using the Bulk API

```
batchforce delete [flags] <SObject> [Expr]
```

### Examples

```

$ batchforce delete --query "SELECT Id FROM Account WHERE Name LIKE Account '%test'"
$ batchforce delete --query "SELECT Id, Name FROM Account" Account 'record.Name matches "\d{3,5}" ? {Id: record.Id} : nil'
	
```

### Options

```
  -b, --batch-size int     batch size.  Set batch size (default: 2000)
  -c, --context string     provide context with anonymous apex
  -n, --dry-run            dry run.  Display updates without modifying records
  -f, --file string        CSV file for input data
      --hard-delete        hard delete records.  Bypass recycle bin and hard delete records
  -h, --help               help for delete
  -q, --query string       SOQL query for input data
      --query-all string   query all records (including archived/deleted records)
  -s, --serialize          serial mode.  Run batch job in Serial mode (default: Parallel)
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
      --quiet              supress informational log messages
```

### SEE ALSO

* [batchforce](batchforce.md)	 - Use Bulk API to update Salesforce records

