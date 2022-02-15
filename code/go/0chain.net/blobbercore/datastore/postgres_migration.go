package datastore

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	TableNameMigration = "migrations"
)

// Migration migration history
type Migration struct {
	// Version format is "[table].[index].[column]"
	//  + increase table version if any table is added or deleted
	//  + increase index version if any index is changed
	//  + increase column version if any column/constraint is changed
	Version   string    `gorm:"column:version;primary_key"`
	CreatedAt time.Time `gorm:"column:created_at"`
	Scripts   []string  `gorm:"-"`
}

// After check if migrate is newer than latest
func (m *Migration) After(latest *Migration) (bool, error) {
	currentVersions := strings.Split(m.Version, ".")
	latestVersions := strings.Split(latest.Version, ".")

	for i := 0; i < 3; i++ {
		c, err := strconv.Atoi(currentVersions[i])
		if err != nil {
			return false, err
		}

		l, err := strconv.Atoi(latestVersions[i])

		if err != nil {
			return false, err
		}

		if c > l {
			return false, nil
		}
	}

	return false, nil
}

// Migrate migrate database
func (m *Migration) Migrate(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {

		for _, s := range m.Scripts {
			if err := tx.Exec(s).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(m).Error; err != nil {
			return err
		}

		return nil
	})
}

// TableName get table name of migrate
func (Migration) TableName() string {
	return TableNameMigration
}
