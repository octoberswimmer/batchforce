## batchforce update

Update Salesforce records using the Bulk API

```
batchforce update [flags] <SObject> <Expr>
```

### Examples

```

$ batchforce update --query "SELECT Id, Name FROM Account WHERE NOT Name LIKE '%test'" Account '{Id: record.Id, Name: record.Name + " Test"}'

$ batchforce update --query "SELECT Id, Type__c FROM Account WHERE RecordType.DeveloperName = 'OldValue'" Account '{Id: record.Id, RecordTypeId: apex.recordTypes[record.Type__c]}' \
  --context "Map<String, Id> recordTypes = new Map<String, Id>(); for (RecordType r : [SELECT DeveloperName, Id FROM RecordType WHERE SobjectType = 'Account']){ recordTypes.put(r.DeveloperName, r.Id); }"
	
```

### Options

```
  -b, --batch-size int     batch size.  Set batch size (default: 2000)
  -c, --context string     provide context with anonymous apex
  -n, --dry-run            dry run.  Display updates without modifying records
  -f, --file string        CSV file for input data
  -h, --help               help for update
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

