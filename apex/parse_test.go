package apex_test

import (
	"testing"

	. "github.com/octoberswimmer/batchforce/apex"
	"github.com/stretchr/testify/assert"
)

func TestLastVar(t *testing.T) {
	var last string
	var err error
	last, err = LastVar(`String x = 'abc';`)
	assert.Nil(t, err)
	assert.Equal(t, "x", last)

	last, err = LastVar(`
		String x = 'abc';
		Integer y = 5;
			`)
	assert.Nil(t, err)
	assert.Equal(t, "y", last)

	last, err = LastVar(`
Integer y = 5;
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
	`)
	assert.Nil(t, err)
	assert.Equal(t, "accounts", last)

	last, err = LastVar(`
Map<String, Integer> numbers = new Map<String, Integer>();
Integer y = 5;
numbers.put('doot', y);
	`)
	assert.Nil(t, err)
	assert.Equal(t, "numbers", last)

	last, err = LastVar(`
Map<String, Integer> numbers = new Map<String, Integer>();
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
Integer y = 5;
numbers.put('doot', y);
JSON.serialize(accounts);
	`)
	assert.Nil(t, err)
	assert.Equal(t, "numbers", last)

	last, err = LastVar(`
Map<String, Integer> numbers = new Map<String, Integer>();
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Id, Name FROM Account]);
Integer y = 5;
numbers.put('doot', y);
JSON.serialize(accounts);
Integer z = 10;
	`)
	assert.Nil(t, err)
	assert.Equal(t, "z", last)
}

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
