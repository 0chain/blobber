package common

import "fmt"

/*IsEmpty checks whether the input string is empty or not */
func IsEmpty(s string) bool {
	return len(s) == 0
}

/*ToKey - takes an interface and returns a Key */
func ToKey(key interface{}) string {
	switch v := key.(type) {
	case string:
		return string(v)
	case []byte:
		return string(v)
	default:
		return string(fmt.Sprintf("%v", v))
	}
}

func IsEqual(key1 string, key2 string) bool {
	return key1 == key2
}
