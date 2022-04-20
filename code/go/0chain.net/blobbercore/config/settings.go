package config

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"go.uber.org/zap"
)

const TableNameSettings = "settings"

type Settings struct {
	ID                      string    `gorm:"column:id;size:10;primaryKey"`
	Capacity                int64     `gorm:"column:capacity;not null;default:0"`
	ChallengeCompletionTime string    `gorm:"column:challenge_completion_time;size:30;default:'-1ns';not null"`
	MaxOfferDuration        string    `gorm:"column:max_offer_duration;size:30;default:'-1ns';not null"`
	MaxStake                int64     `gorm:"column:max_stake;not null;default:100"`
	MinLockDemand           float64   `gorm:"column:min_lock_demand;not null;default:0"`
	MinStake                int64     `gorm:"column:min_lock_demand;not null;default:1"`
	NumDelegates            int       `gorm:"column:num_delegates;not null;default:100"`
	ReadPrice               float64   `gorm:"column:read_price;not null;default:0"`
	WritePrice              float64   `gorm:"column:write_price;not null;default:0"`
	ServiceCharge           float64   `gorm:"column:service_charge;not null;default:0"`
	UpdatedAt               time.Time `gorm:"column:updated_at;not null;default:NOW()"`
}

func (s Settings) TableName() string {
	return TableNameSettings
}

// ToConfiguration copy settings to config.Configuration
func (s *Settings) ToConfiguration() error {

	if s == nil {
		return errors.Throw(constants.ErrInvalidParameter, "s")
	}

	Configuration.Capacity = s.Capacity
	cct, err := time.ParseDuration(s.ChallengeCompletionTime)
	if err != nil {
		return errors.Throw(constants.ErrInvalidParameter, "ChallengeCompletionTime")
	}
	Configuration.ChallengeCompletionTime = cct

	maxOfferDuration, err := time.ParseDuration(s.MaxOfferDuration)
	if err != nil {
		return errors.Throw(constants.ErrInvalidParameter, "MaxOfferDuration")
	}
	Configuration.MaxOfferDuration = maxOfferDuration
	Configuration.MaxStake = s.MaxStake
	Configuration.MinLockDemand = s.MinLockDemand
	Configuration.MinStake = s.MinStake
	Configuration.NumDelegates = s.NumDelegates
	Configuration.ReadPrice = s.ReadPrice
	Configuration.ServiceCharge = s.ServiceCharge
	Configuration.WritePrice = s.WritePrice

	return nil
}

// FromConfiguration copy settings from config.Configuration
func (s *Settings) FromConfiguration() error {
	if s == nil {
		return errors.Throw(constants.ErrInvalidParameter, "s")
	}

	s.Capacity = Configuration.Capacity
	s.ChallengeCompletionTime = Configuration.ChallengeCompletionTime.String()
	s.MaxOfferDuration = Configuration.MaxOfferDuration.String()
	s.MaxStake = Configuration.MaxStake
	s.MinLockDemand = Configuration.MinLockDemand
	s.MinStake = Configuration.MinStake
	s.NumDelegates = Configuration.NumDelegates
	s.ReadPrice = Configuration.ReadPrice
	s.ServiceCharge = Configuration.ServiceCharge
	s.WritePrice = Configuration.WritePrice

	return nil
}

// Get load settings
func Get(ctx context.Context) (*Settings, bool) {
	db := datastore.GetStore().GetDB()

	var s Settings
	if err := db.Table(TableNameSettings).
		Where(`id="settings"`).
		First(&s).Error; err == nil {
		return &s, true
	}
	return nil, false

}

// Update update settings in db
func Update(ctx context.Context) error {
	s := &Settings{}

	if err := s.FromConfiguration(); err != nil {
		return err
	}

	db := datastore.GetStore().GetDB()

	s.UpdatedAt = time.Now()
	if s.ID == "settings" {
		if err := db.Save(s).Error; err != nil {
			return err
		}

		return nil
	}

	s.ID = "settings"
	if err := db.Create(s).Error; err != nil {
		return err
	}

	return nil
}

// Refresh sync latest settings from blockchain
func Refresh(ctx context.Context) error {
	b, err := sdk.GetBlobber(node.Self.ID)
	if err != nil { // blobber is not registered yet
		logging.Logger.Warn("failed to sync blobber settings from blockchain", zap.Error(err))

		return err
	}

	Configuration.Capacity = int64(b.Capacity)
	Configuration.Capacity = int64(b.Capacity)
	Configuration.ChallengeCompletionTime = b.Terms.ChallengeCompletionTime
	Configuration.MaxOfferDuration = b.Terms.MaxOfferDuration
	Configuration.MaxStake = int64(b.StakePoolSettings.MaxStake)
	Configuration.MinLockDemand = b.Terms.MinLockDemand
	Configuration.MinStake = int64(b.StakePoolSettings.MinStake)
	Configuration.NumDelegates = b.StakePoolSettings.NumDelegates
	Configuration.ReadPrice = float64(b.Terms.ReadPrice)
	Configuration.WritePrice = float64(b.Terms.WritePrice)
	Configuration.WritePrice = b.StakePoolSettings.ServiceCharge

	return Update(ctx)
}
