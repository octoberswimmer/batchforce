## batchforce upsert

upsert Salesforce records using the Bulk API

```
batchforce upsert [flags] <SObject> <Expr>
```

### Examples

```

$ batchforce upsert --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Name: record.Name + " Copy"}'
$ batchforce upsert --file accounts.csv Account '{Name: record.Name + " Copy"}'
	
```

### Options

```
  -b, --batch-size int       batch size.  Set batch size (default: 2000)
  -c, --context string       provide context with anonymous apex
  -n, --dry-run              dry run.  Display updates without modifying records
  -e, --external-id string   external id
  -f, --file string          CSV file for input data
  -h, --help                 help for upsert
  -q, --query string         SOQL query for input data
      --query-all string     query all records (including archived/deleted records)
  -s, --serialize            serial mode.  Run batch job in Serial mode (default: Parallel)
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
      --quiet              suppress informational log messages
```

### SEE ALSO

* [batchforce](batchforce.md)	 - Use Bulk API to update Salesforce records

