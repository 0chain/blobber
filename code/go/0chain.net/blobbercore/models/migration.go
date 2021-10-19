package models

import (
	"errors"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	TableNameMigration = "migrations"
)

type Migration struct {
	// Version format is "[table].[index].[column]"
	//  + increase table version if any table is added or deleted
	//  + increase index version if any index is changed
	//  + increase column version if any column/constraint is changed
	Version   string    `gorm:"column:version;primary_key"`
	CreatedAt time.Time `gorm:"column:created_at"`
	Scripts   []string  `gorm:"-"`
}

func (m *Migration) After(latest *Migration) bool {
	currentVersions := strings.Split(m.Version, ".")
	latestVersions := strings.Split(latest.Version, ".")

	for i := 0; i < 3; i++ {
		if currentVersions[i] > latestVersions[i] {
			return true
		}
	}

	return false
}

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

func (Migration) TableName() string {
	return TableNameMigration
}

func AutoMigrate(db *gorm.DB) {

	err := db.AutoMigrate(&Migration{})
	if err != nil {
		logging.Logger.Error("[db]", zap.Error(err))
	}

	latest := &Migration{}
	result := db.Raw(`select * from "migrations" order by "version" desc limit 1`).First(latest)

	if result.Error != nil {
		if errors.Is(gorm.ErrRecordNotFound, result.Error) {
			latest.Version = "0.0.0"
			latest.CreatedAt = time.Date(2021, 10, 14, 0, 0, 0, 0, time.UTC)
			err = db.Create(latest).Error

			if err != nil {
				logging.Logger.Error("[db]"+latest.Version, zap.Error(err))
				return
			}
		} else {
			logging.Logger.Error("[db]", zap.Error(result.Error))
			return
		}
	}

	for i := 0; i < len(releases); i++ {
		v := releases[i]
		if v.After(latest) {
			err = v.Migrate(db)
			if err != nil {
				logging.Logger.Error("[db]"+v.Version, zap.Error(err))
			} else {
				logging.Logger.Info("[db]" + v.Version + " migrated")
			}
		}
	}

}

var releases = []Migration{
	{
		Version:   "0.1.0",
		CreatedAt: time.Date(2021, 10, 15, 0, 0, 0, 0, time.UTC),
		Scripts: []string{
			"CREATE INDEX idx_allocation_path ON reference_objects (allocation_id,path,deleted_at);",
		},
	},
}
