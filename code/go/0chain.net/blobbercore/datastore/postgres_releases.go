package datastore

import "time"

// releases migration histories
var releases = []Migration{
	{
		Version:   "0.0.1",
		Remark:    "Added available_at on marketplace_share_info",
		CreatedAt: time.Date(2022, 3, 21, 0, 0, 0, 0, time.UTC),
		Scripts: []string{
			`ALTER TABLE marketplace_share_info
					ADD COLUMN IF NOT EXISTS available_at timestamp without time zone NOT NULL DEFAULT now();`,
		},
	},
}
