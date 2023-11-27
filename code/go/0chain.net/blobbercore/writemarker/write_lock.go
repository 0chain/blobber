package writemarker

import "time"

// WriteLock WriteMarker lock
type WriteLock struct {
	ConnectionID string    `gorm:"column:connection_id;size:64"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}
