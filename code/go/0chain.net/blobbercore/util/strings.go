package util

import (
	"strings"
)

// SplitFiles split files from a path
func SplitFiles(path string) []string {
	files := make([]string, 0)
	for _, it := range strings.Split(path, "/") {
		if len(it) > 0 {
			files = append(files, it)
		}
	}

	return files
}
