package datastore

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// UseInMemory set the DB instance to an in-memory DB using SQLite.
func UseInMemory() (*gorm.DB, error) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	instance = &postgresStore{
		db: gdb,
	}

	return gdb, nil
}
