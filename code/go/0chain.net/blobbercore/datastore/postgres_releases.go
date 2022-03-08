package datastore

import "time"

// releases migration histories
var releases = []Migration{
	{
		Version:   "0.1.0",
		CreatedAt: time.Date(2022, 0, 30, 0, 0, 0, 0, time.UTC),
		Scripts: []string{
			`ALTER TABLE write_markers 
			ADD name varchar(255),
			ADD lookup_hash varchar(255),
			ADD content_hash varchar(255);`,
		},
	},
}
