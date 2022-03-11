package common

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IsEmpty checks whether the input string is empty or not
func IsEmpty(s string) bool {
	return s == ""
}

// ToKey - takes an interface and returns a Key
func ToKey(key interface{}) string {
	switch v := key.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func IsEqual(key1, key2 string) bool {
	return key1 == key2
}

// getParentPaths For path /a/b/c.txt, will return [/a,/a/b]
func GetParentPaths(fPath string) ([]string, error) {
	if fPath == "" {
		return nil, nil
	}

	fPath = filepath.Clean(fPath)
	if !filepath.IsAbs(fPath) {
		return nil, NewError("invalid_path", fmt.Sprintf("%v is not absolute path", fPath))
	}
	splittedPaths := strings.Split(fPath, "/")
	var paths []string

	for i := 0; i < len(splittedPaths); i++ {
		subPath := strings.Join(splittedPaths[0:i], "/")
		paths = append(paths, subPath)
	}
	return paths[2:], nil
}
