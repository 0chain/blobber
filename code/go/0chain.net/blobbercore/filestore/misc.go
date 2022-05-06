package filestore

import (
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
)

// TODO make return values uint
var getDirLevelsForAllocations = func() []int {
	return []int{2, 1} // default
}

var getDirLevelsForFiles = func() []int {
	return []int{2, 2, 1} // default
}

func validateDirLevels() error {
	if len(config.Configuration.AllocDirLevel) > 0 {
		getDirLevelsForAllocations = func() []int { return config.Configuration.AllocDirLevel }
	}

	if len(config.Configuration.FileDirLevel) > 0 {
		getDirLevelsForFiles = func() []int { return config.Configuration.FileDirLevel }
	}

	var s int
	for _, i := range getDirLevelsForAllocations() {
		s += i
	}
	if s >= 64 || s <= 0 {
		return errors.New("allocation directory levels sum should be in range 0<s<=64")
	}

	s = 0
	for _, i := range getDirLevelsForFiles() {
		s += i
	}
	if s >= 64 || s <= 0 {
		return errors.New("files directory levels has sum should be in range 0<s<=64")
	}

	return nil
}
