package apex_test

import (
	"testing"

	. "github.com/octoberswimmer/batchforce/apex"
	"github.com/stretchr/testify/assert"
)

func TestAllVars(t *testing.T) {
	var last []string
	var err error
	last, err = Vars(`String x = 'abc';`)
	assert.Nil(t, err)
	assert.Equal(t, []string{"x"}, last)

	last, err = Vars(`
		String x = 'abc';
		Integer y = 5;
			`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"x", "y"}, last)

	last, err = Vars(`
Integer y = 5;
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"y", "accounts"}, last)

	last, err = Vars(`
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
Integer y = 5;
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"y", "accounts"}, last)

	last, err = Vars(`
Map<String, Integer> numbers = new Map<String, Integer>();
Integer y = 5;
numbers.put('doot', y);
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"numbers", "y"}, last)

	last, err = Vars(`
Map<String, Integer> numbers = new Map<String, Integer>();
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
Integer y = 5;
numbers.put('doot', y);
JSON.serialize(accounts);
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"numbers", "accounts", "y"}, last)

	last, err = Vars(`
Map<String, Integer> numbers = new Map<String, Integer>();
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
Integer y = 5;
numbers.put('doot', y);
JSON.serialize(accounts);
Integer z = 10;
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"numbers", "accounts", "y", "z"}, last)
}

func TestVarsIgnoresNestedScopes(t *testing.T) {
	last, err := Vars(`
Map<String, Zip_Code__c> zipMap = new Map<String, Zip_Code__c>();
for (Zip_Code__c z : [SELECT Name FROM Zip_Code__c]) {
	String k = z.Name.trim();
	if (k.length() > 5) { k = k.substring(0, 5); }
	if (!zipMap.containsKey(k)) { zipMap.put(k, z); }
}
	`)
	assert.Nil(t, err)
	assert.Equal(t, []string{"zipMap"}, last)

	last, err = Vars(`
Integer total = 0;
if (total == 0) {
	Integer hidden = 1;
	total += hidden;
}
while (total < 10) {
	Integer step = 2;
	total += step;
}
{
	Integer scoped = 3;
}
String kept = 'x';
	`)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"total", "kept"}, last)
}
