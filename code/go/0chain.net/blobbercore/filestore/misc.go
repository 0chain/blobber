package filestore

import (
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
)

var getDirLevelsForAllocations = func() []int {
	return []int{2, 1} // default
}

var getDirLevelsForFiles = func() []int {
	return []int{2, 2, 1} // default
}

func validateDirLevels() error {
	if config.Configuration.AllocDirLevel != nil {
		getDirLevelsForAllocations = func() []int { return config.Configuration.AllocDirLevel }
	}

	if config.Configuration.FileDirLevel != nil {
		getDirLevelsForFiles = func() []int { return config.Configuration.FileDirLevel }
	}

	var s int
	for _, i := range getDirLevelsForAllocations() {
		s += i
	}
	if s >= 64 {
		return errors.New("allocation directory levels has sum greater than 64")
	}

	s = 0
	for _, i := range getDirLevelsForFiles() {
		s += i
	}
	if s >= 64 {
		return errors.New("files directory levels has sum greater than 64")
	}

	return nil
}
