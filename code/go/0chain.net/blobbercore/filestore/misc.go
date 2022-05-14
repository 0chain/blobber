package filestore

import (
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
)

// For an allocation 4c9bad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56, providing dirlevel [1,2] would
// return string {mount_point}/4/c9/bad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56
// Similarly for dirlevel [1,2,3,4] it would return
// {mount_point}/4/c9/bad/2522/72bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56
var getDirLevelsForAllocations = func() []int { // TODO make return values uint
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
