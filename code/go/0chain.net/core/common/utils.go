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

// GetPathFields will return slice of fields of path.
// For path /a/b/c/d/e/f.txt it will return [a, b, c, d, e, f.txt],nil
func GetPathFields(p string) ([]string, error) {
	if p == "" || p == "/" {
		return nil, nil
	}

	if !filepath.IsAbs(p) {
		return nil, NewError("invalid_path", fmt.Sprintf("%v is not absolute path", p))
	}

	p = filepath.Clean(p)
	fields := strings.Split(p, "/")
	return fields[1:], nil
}
