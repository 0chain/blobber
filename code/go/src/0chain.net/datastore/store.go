package datastore

import (
	"context"
	"fmt"
)

/*Key - a type for the entity key */
type Key = string

type Store interface {
	Read(ctx context.Context, key Key, entity Entity) error
	Write(ctx context.Context, entity Entity) error
	Delete(ctx context.Context, entity Entity) error

	MultiRead(ctx context.Context, entityMetadata EntityMetadata, keys []Key, entities []Entity) error
	MultiWrite(ctx context.Context, entityMetadata EntityMetadata, entities []Entity) error
	MultiDelete(ctx context.Context, entityMetadata EntityMetadata, entities []Entity) error
}

/*ToString - return string representation of the key */
func ToString(key Key) string {
	return string(key)
}

func IsEmpty(key Key) bool {
	return len(key) == 0
}

func IsEqual(key1 Key, key2 Key) bool {
	return key1 == key2
}

/*EmptyKey - Represents an empty key */
var EmptyKey = Key("")

/*ToKey - takes an interface and returns a Key */
func ToKey(key interface{}) Key {
	switch v := key.(type) {
	case string:
		return Key(v)
	case []byte:
		return Key(v)
	default:
		return Key(fmt.Sprintf("%v", v))
	}
}
