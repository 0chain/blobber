package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

// FileChanger file change processor
type FileChanger interface {
	// ProcessChange process change and save them on reference_objects
	ProcessChange(ctx context.Context,
		change *AllocationChange, allocationRoot string) (*reference.Ref, error)
	// Marshal marshal change as JSON string
	Marshal() (string, error)
	// Unmarshal unmarshal change from JSON string
	Unmarshal(input string) error
	// DeleteTempFile delete temp file and thumbnail from disk
	DeleteTempFile() error
	// CommitToFileStore move temp file and thumbnail from temp dir to persistent folder
	CommitToFileStore(ctx context.Context) error
}
