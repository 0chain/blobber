package stats

import (
	"database/sql"
)

// DBStats contains database statistics.
type DBStats struct {
	sql.DBStats
	Status string
}
