package reference

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"gorm.io/gorm"
)

type ShareInfo struct {
	ID                        int       `gorm:"column:id;primaryKey"`
	OwnerID                   string    `gorm:"column:owner_id;size:64;not null;index:idx_marketplace_share_info_for_owner,priority:1" json:"owner_id,omitempty"`
	ClientID                  string    `gorm:"column:client_id;size:64;not null;index:idx_marketplace_share_info_for_client,priority:1" json:"client_id"`
	FilePathHash              string    `gorm:"column:file_path_hash;size:64;not null;index:idx_marketplace_share_info_for_owner,priority:2;index:idx_marketplace_share_info_for_client,priority:2" json:"file_path_hash,omitempty"`
	ReEncryptionKey           string    `gorm:"column:re_encryption_key;not null" json:"re_encryption_key,omitempty"`
	ClientEncryptionPublicKey string    `gorm:"column:client_encryption_public_key;not null" json:"client_encryption_public_key,omitempty"`
	Revoked                   bool      `gorm:"column:revoked;not null" json:"revoked"`
	ExpiryAt                  time.Time `gorm:"column:expiry_at;not null" json:"expiry_at,omitempty"`
	AvailableAt               time.Time `gorm:"column:available_at;type:timestamp without time zone;not null;default:now()" json:"available_at,omitempty"`
}

func (ShareInfo) TableName() string {
	return "marketplace_share_info"
}

// add share if it already doesnot exist
func AddShareInfo(ctx context.Context, shareInfo ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Model(&ShareInfo{}).Create(shareInfo).Error
}

func DeleteShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)

	result := db.Model(&ShareInfo{}).
		Where(&ShareInfo{
			ClientID:     shareInfo.ClientID,
			FilePathHash: shareInfo.FilePathHash,
			Revoked:      false,
		}).
		Updates(ShareInfo{
			Revoked: true,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateShareInfo(ctx context.Context, shareInfo ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)

	return db.Model(&ShareInfo{}).
		Where(&ShareInfo{
			ClientID:     shareInfo.ClientID,
			FilePathHash: shareInfo.FilePathHash,
		}).
		Select("Revoked", "ReEncryptionKey", "ExpiryAt", "AvailableAt", "ClientEncryptionPublicKey").
		Updates(shareInfo).
		Error
}

func GetShareInfo(ctx context.Context, clientID, filePathHash string) (*ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	shareInfo := &ShareInfo{}
	err := db.Model(&ShareInfo{}).
		Where(&ShareInfo{
			ClientID:     clientID,
			FilePathHash: filePathHash,
		}).
		First(shareInfo).Error

	if err != nil {
		return nil, err
	}
	return shareInfo, nil
}
