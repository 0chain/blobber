package writemarker

import "time"

// WriteLock WriteMarker lock
type WriteLock struct {
	ConnectionID string
	CreatedAt    time.Time
}
