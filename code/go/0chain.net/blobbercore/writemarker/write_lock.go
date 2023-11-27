package writemarker

import "time"

const (
	TableNameWriteLock = "write_locks"
)

// WriteLock WriteMarker lock
type WriteLock struct {
	ConnectionID string    `gorm:"column:connection_id;size:64"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

// TableName get table name of migrate
func (WriteLock) TableName() string {
	return TableNameWriteLock
}
