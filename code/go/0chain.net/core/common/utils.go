package common

import "fmt"

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
