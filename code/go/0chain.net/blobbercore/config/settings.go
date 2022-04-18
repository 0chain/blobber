package config

import (
	"context"

	"gorm.io/gorm"
)

const TableNameSettings = "settings"

type Settings struct {
	ID                      string  `gorm:"column:id;size:10;primaryKey"`
	Capacity                int64   `gorm:"column:capacity;not null;default:0"`
	ChallengeCompletionTime string  `gorm:"column:challenge_completion_time;size:30;default:'-1ns';not null"`
	MaxOfferDuration        string  `gorm:"column:max_offer_duration;size:30;default:'-1ns';not null"`
	MaxStake                int64   `gorm:"column:max_stake;not null;default:100"`
	MinLockDemand           float64 `gorm:"column:min_lock_demand;not null;default:0"`
	MinStake                int64   `gorm:"column:min_lock_demand;not null;default:1"`
	NumDelegates            int     `gorm:"column:num_delegates;not null;default:100"`
	ReadPrice               float64 `gorm:"column:read_price;not null;default:0"`
	WritePrice              float64 `gorm:"column:write_price;not null;default:0"`
	ServiceCharge           float64 `gorm:"column:service_charge;not null;default:0"`
}

func (s Settings) TableName() string {
	return TableNameSettings
}

// Get load settings
func Get(ctx context.Context, db *gorm.DB) (*Settings, bool) {
	var s Settings
	if err := db.Table(TableNameSettings).
		Where(`id="settings"`).
		First(&s).Error; err == nil {
		return &s, true
	}
	return nil, false

}

// Update settings
func Update(ctx context.Context, db *gorm.DB, s *Settings) error {

	if s.ID == "settings" {
		return db.Save(s).Error
	}

	s.ID = "settings"
	return db.Create(s).Error
}
