## batchforce insert

insert Salesforce records using the Bulk API

```
batchforce insert [flags] <SObject> <Expr>
```

### Examples

```

$ batchforce insert --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Name: record.Name + " Copy"}'
$ batchforce insert --file accounts.csv Account '{Name: record.Name + " Copy"}'
	
```

### Options

```
  -b, --batch-size int     batch size.  Set batch size (default: 2000)
  -c, --context string     provide context with anonymous apex
  -n, --dry-run            dry run.  Display updates without modifying records
  -f, --file string        CSV file for input data
  -h, --help               help for insert
  -q, --query string       SOQL query for input data
      --query-all string   query all records (including archived/deleted records)
  -s, --serialize          serial mode.  Run batch job in Serial mode (default: Parallel)
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
      --help-expr          show expr language definition
      --quiet              suppress informational log messages
```

### SEE ALSO

* [batchforce](batchforce.md)	 - Use Bulk API to update Salesforce records

