package reference

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"gorm.io/gorm"
)

type ShareInfo struct {
	OwnerID                   string    `gorm:"owner_id" json:"owner_id,omitempty"`
	ClientID                  string    `gorm:"client_id" json:"client_id"`
	FilePathHash              string    `gorm:"file_path_hash" json:"file_path_hash,omitempty"`
	ReEncryptionKey           string    `gorm:"re_encryption_key" json:"re_encryption_key,omitempty"`
	ClientEncryptionPublicKey string    `gorm:"client_encryption_public_key" json:"client_encryption_public_key,omitempty"`
	Revoked                   bool      `gorm:"revoked" json:"revoked"`
	ExpiryAt                  time.Time `gorm:"expiry_at" json:"expiry_at,omitempty"`
	AvailableAt               time.Time `gorm:"available_at" json:"available_at,omitempty"`
}

func TableName() string {
	return "marketplace_share_info"
}

// add share if it already doesnot exist
func AddShareInfo(ctx context.Context, shareInfo ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Table(TableName()).Create(shareInfo).Error
}

func DeleteShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)

	result := db.Table(TableName()).
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

	return db.Table(TableName()).
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
	err := db.Table(TableName()).
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
