package datastore

import (
	"context"
	"fmt"
)

/*Key - a type for the entity key */
type Key = string

/*StoreIteratorHandler is a iteration handler function type */
type StoreIteratorHandler func(ctx context.Context, key Key, value []byte) error

type Store interface {
	Read(ctx context.Context, key Key, entity Entity) error
	Write(ctx context.Context, entity Entity) error
	Delete(ctx context.Context, entity Entity) error
	DeleteKey(ctx context.Context, key Key) error
	ReadBytes(ctx context.Context, key Key) ([]byte, error)
	WriteBytes(ctx context.Context, key Key, value []byte) error

	MultiRead(ctx context.Context, entityMetadata EntityMetadata, keys []Key, entities []Entity) error
	MultiWrite(ctx context.Context, entityMetadata EntityMetadata, entities []Entity) error
	MultiDelete(ctx context.Context, entityMetadata EntityMetadata, entities []Entity) error
	Iterate(ctx context.Context, iter StoreIteratorHandler) error
	IteratePrefix(ctx context.Context, prefix string, iter StoreIteratorHandler) error
	WithConnection(ctx context.Context) context.Context
	WithReadOnlyConnection(ctx context.Context) context.Context
	Commit(ctx context.Context) error
	Discard(ctx context.Context)
}

const CONNECTION_CONTEXT_KEY = "connection"

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
