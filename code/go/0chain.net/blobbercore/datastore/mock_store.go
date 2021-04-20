package datastore

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
)

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

	setDB(gdb)

	return mock
}
