//This file is used to create table schemas using gorm's automigration feature which takes information from
//struct's fields and functions

package automigration

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"gorm.io/gorm"
)

type tableNameI interface {
	TableName() string
}

var tableModels = []tableNameI{
	new(reference.Ref),
	new(reference.ShareInfo),
	new(challenge.ChallengeEntity),
	new(challenge.ChallengeTiming),
	new(allocation.Allocation),
	new(allocation.AllocationChange),
	new(allocation.AllocationChangeCollector),
	new(allocation.Pending),
	new(allocation.Terms),
	new(allocation.ReadPool),
	new(allocation.WritePool),
	new(readmarker.ReadMarkerEntity),
	new(writemarker.WriteMarkerEntity),
	new(writemarker.WriteLock),
	new(reference.FileStats),
	new(config.Settings),
}

func AutoMigrate(pgDB *gorm.DB) error {
	if err := createUser(pgDB); err != nil {
		return err
	}

	if err := createDB(pgDB); err != nil {
		return err
	}

	if err := grantPrivileges(pgDB); err != nil {
		return err
	}

	d, err := pgDB.DB()
	if err != nil {
		return err
	}

	if err := d.Close(); err != nil {
		return err
	}

	if err := datastore.GetStore().Open(); err != nil {
		return err
	}

	db := datastore.GetStore().GetDB()
	return MigrateSchema(db)
}

func createDB(db *gorm.DB) (err error) {
	// check if db exists
	dbstmt := fmt.Sprintf("SELECT datname, oid FROM pg_database WHERE datname = '%s';", config.Configuration.DBName)
	rs := db.Raw(dbstmt)
	if rs.Error != nil {
		return rs.Error
	}

	var result struct {
		Datname string
	}

	if rs.Scan(&result); len(result.Datname) == 0 {
		stmt := fmt.Sprintf("CREATE DATABASE %s;", config.Configuration.DBName)
		if rs := db.Exec(stmt); rs.Error != nil {
			return rs.Error
		}
		if rs := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm;"); rs.Error != nil {
			return rs.Error
		}
	}
	return
}

func createUser(db *gorm.DB) error {
	usrstmt := fmt.Sprintf("SELECT usename, usesysid FROM pg_catalog.pg_user WHERE usename = '%s';", config.Configuration.DBUserName)
	rs := db.Raw(usrstmt)
	if rs.Error != nil {
		return rs.Error
	}

	var result struct {
		Usename string
	}

	if rs.Scan(&result); len(result.Usename) == 0 {
		stmt := fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD '%s';", config.Configuration.DBUserName, config.Configuration.DBPassword)
		if rs := db.Exec(stmt); rs.Error != nil && rs.Error.Error() != fmt.Sprintf("pq: role \"%s\" already exists", config.Configuration.DBUserName) {
			return rs.Error
		}
	}
	return nil
}

func grantPrivileges(db *gorm.DB) error {
	stmts := []string{
		fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;", config.Configuration.DBName, config.Configuration.DBUserName),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s;", config.Configuration.DBUserName),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s;", config.Configuration.DBUserName),
	}
	for _, stmt := range stmts {
		err := db.Exec(stmt).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func MigrateSchema(db *gorm.DB) error {
	var tables []interface{} // Put in new slice to resolve type mismatch
	for _, tbl := range tableModels {
		tables = append(tables, tbl)
	}

	if err := db.AutoMigrate(tables...); err != nil {
		return err
	}
	err := db.Exec(`ALTER TABLE reference_objects ALTER COLUMN path TYPE varchar(1000) COLLATE "POSIX"`).Error
	if err != nil {
		return err
	}
	return nil
}

// DropSchemas is used for integration tests to clear DB.
func DropSchemas(db *gorm.DB) error {
	var tables []interface{} // Put in new slice to resolve type mismatch
	for _, tbl := range tableModels {
		tables = append(tables, tbl)
	}

	return db.Migrator().DropTable(tables...)
}
