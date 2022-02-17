package datastore

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func MocketTheStore(t *testing.T, logging bool) {
	var err error

	mocket.Catcher.Reset()
	mocket.Catcher.Register()
	mocket.Catcher.Logging = logging

	dialect := postgres.New(postgres.Config{
		DSN:                  "mockdb",
		DriverName:           mocket.DriverName,
		PreferSimpleProtocol: true,
	})

	gdb, err := gorm.Open(dialect, new(gorm.Config))
	require.NoError(t, err)

	instance = &postgresStore{
		db: gdb,
	}
}

// sqlmock has problems with inserts, so use mocket for tests with inserts

func MockTheStore(t *testing.T) sqlmock.Sqlmock {
	var db *sql.DB
	var mock sqlmock.Sqlmock
	var err error
	db, mock, err = sqlmock.New()
	require.NoError(t, err)

	var dialector = postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})
	var gdb *gorm.DB
	gdb, err = gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	instance = &postgresStore{
		db: gdb,
	}

	return mock
}
