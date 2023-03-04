package config

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/zcn"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const TableNameSettings = "settings"

type Settings struct {
	ID               string    `gorm:"column:id;size:10;primaryKey"`
	Capacity         int64     `gorm:"column:capacity;not null;default:0"`
	MaxOfferDuration string    `gorm:"column:max_offer_duration;size:30;default:'-1ns';not null"`
	MaxStake         int64     `gorm:"column:max_stake;not null;default:100"`
	MinLockDemand    float64   `gorm:"column:min_lock_demand;not null;default:0"`
	MinStake         int64     `gorm:"column:min_stake;not null;default:1"`
	NumDelegates     int       `gorm:"column:num_delegates;not null;default:100"`
	ReadPrice        float64   `gorm:"column:read_price;not null;default:0"`
	WritePrice       float64   `gorm:"column:write_price;not null;default:0"`
	ServiceCharge    float64   `gorm:"column:service_charge;not null;default:0"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null;default:current_timestamp"`
}

func (s Settings) TableName() string {
	return TableNameSettings
}

// CopyTo copy settings to config.Configuration
func (s *Settings) CopyTo(c *Config) error {

	if s == nil {
		return errors.Throw(constants.ErrInvalidParameter, "s")
	}

	c.Capacity = s.Capacity

	maxOfferDuration, err := time.ParseDuration(s.MaxOfferDuration)
	if err != nil {
		return errors.Throw(constants.ErrInvalidParameter, "MaxOfferDuration")
	}
	c.MaxOfferDuration = maxOfferDuration
	c.MaxStake = s.MaxStake
	c.MinLockDemand = s.MinLockDemand
	c.MinStake = s.MinStake
	c.NumDelegates = s.NumDelegates
	c.ReadPrice = s.ReadPrice
	c.ServiceCharge = s.ServiceCharge
	c.WritePrice = s.WritePrice

	return nil
}

// CopyFrom copy settings from config.Configuration
func (s *Settings) CopyFrom(c *Config) error {
	if s == nil {
		return errors.Throw(constants.ErrInvalidParameter, "s")
	}

	s.Capacity = c.Capacity
	s.MaxOfferDuration = c.MaxOfferDuration.String()
	s.MaxStake = c.MaxStake
	s.MinLockDemand = c.MinLockDemand
	s.MinStake = c.MinStake
	s.NumDelegates = c.NumDelegates
	s.ReadPrice = c.ReadPrice
	s.ServiceCharge = c.ServiceCharge
	s.WritePrice = c.WritePrice

	return nil
}

// Get load settings
func Get(ctx context.Context, db *gorm.DB) (*Settings, bool) {
	if db == nil {
		return nil, false
	}
	var s Settings
	if err := db.Table(TableNameSettings).
		Where(`id=?`, "settings").
		First(&s).Error; err == nil {
		return &s, true
	}
	return nil, false

}

// Update update settings in db
func Update(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.Throw(constants.ErrInvalidParameter, "db")
	}
	s, ok := Get(ctx, db)
	if !ok {
		s = &Settings{
			ID: "settings",
		}
	}

	s.UpdatedAt = time.Now()
	if err := s.CopyFrom(&Configuration); err != nil {
		return err
	}

	if ok {
		return db.Save(s).Error
	}

	return db.Create(s).Error
}

// ReloadFromChain load and refresh latest settings from blockchain
func ReloadFromChain(ctx context.Context, db *gorm.DB) (*zcncore.Blobber, error) {
	if db == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "db")
	}

	b, err := zcn.GetBlobber(node.Self.ID)
	if err != nil { // blobber is not registered yet
		logging.Logger.Warn("failed to sync blobber settings from blockchain", zap.Error(err))

		return nil, err
	}

	Configuration.Capacity = int64(b.Capacity)
	Configuration.MaxOfferDuration = b.Terms.MaxOfferDuration
	Configuration.MaxStake = int64(b.StakePoolSettings.MaxStake)
	Configuration.MinLockDemand = b.Terms.MinLockDemand
	Configuration.MinStake = int64(b.StakePoolSettings.MinStake)
	Configuration.NumDelegates = b.StakePoolSettings.NumDelegates
	Configuration.ReadPrice, err = b.Terms.ReadPrice.ToToken()
	if err != nil {
		logging.Logger.Error("Invalid read price config", zap.Error(err))
	}
	Configuration.WritePrice, err = b.Terms.WritePrice.ToToken()
	if err != nil {
		logging.Logger.Error("Invalid write price config", zap.Error(err))
	}
	Configuration.ServiceCharge = b.StakePoolSettings.ServiceCharge

	return b, Update(ctx, db)
}
