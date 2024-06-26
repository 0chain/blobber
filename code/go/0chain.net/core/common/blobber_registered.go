package common

import "sync"

// Private variables
var (
	blobberRegisteredMutex sync.Mutex
	blobberRegistered      bool
)

// Getter for blobberRegistered
func IsBlobberRegistered() bool {
	blobberRegisteredMutex.Lock()
	defer blobberRegisteredMutex.Unlock()
	return blobberRegistered
}

// Setter for blobberRegistered
func SetBlobberRegistered(registered bool) {
	blobberRegisteredMutex.Lock()
	defer blobberRegisteredMutex.Unlock()
	blobberRegistered = registered
}
