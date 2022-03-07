package util

import (
	"strings"
)

// SplitPath split files from a path
func SplitPath(path string) []string {
	items := make([]string, 0)
	for _, it := range strings.Split(path, "/") {
		if len(it) > 0 {
			items = append(items, it)
		}
	}

	return items
}
