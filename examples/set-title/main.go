package main

import (
	force "github.com/ForceCLI/force/lib"
	batch "github.com/octoberswimmer/batchforce"
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
