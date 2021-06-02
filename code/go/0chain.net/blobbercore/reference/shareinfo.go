package reference

import (
	"0chain.net/blobbercore/datastore"
	"context"
	"time"
)


type ShareInfo struct {
	OwnerID                       string          `gorm:"owner_id" json:"owner_id,omitempty"`
	ClientID                      string          `gorm:"client_id" json:"client_id"`
	FileName                      string          `gorm:"file_name" json:"file_name,omitempty"`
	ReEncryptionKey               string          `gorm:"re_encryption_key" json:"re_encryption_key,omitempty"`
	ClientEncryptionPublicKey     string          `gorm:"client_encryption_public_key" json:"client_encryption_public_key,omitempty"`
	ExpiryAt                      time.Time       `gorm:"expiry_at" json:"expiry_at,omitempty"`
}

func TableName() string {
	return "marketplace_share_info"
}

func AddShareInfo(ctx context.Context, shareInfo ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Table(TableName()).Create(shareInfo).Error
}

func GetShareInfo(ctx context.Context, clientID string, fileName string) (ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	shareInfo := ShareInfo{}
	err := db.Table(TableName()).
		Where(&ShareInfo{
			ClientID:    clientID,
			FileName: fileName,
		}).
		First(&shareInfo).Error

	return shareInfo, err
}