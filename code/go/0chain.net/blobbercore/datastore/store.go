package datastore

import (
	"context"

	"gorm.io/gorm"
)

type contextKey int

const (
	ContextKeyTransaction contextKey = iota
	ContextKeyStore
)

type Store interface {

	// GetDB get raw gorm db
	GetDB() *gorm.DB
	// CreateTransaction create transaction, and save it in context
	CreateTransaction(ctx context.Context) context.Context
	// GetTransaction get transaction from context
	GetTransaction(ctx context.Context) *gorm.DB

	Open() error
	Close()
}

var instance Store

func GetStore() Store {
	return instance
}

func FromContext(ctx context.Context) Store {
	store := ctx.Value(ContextKeyStore)
	if store != nil {
		return store.(Store)
	}

	return GetStore()
}
