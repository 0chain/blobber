package reference

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm"
)

// ShareType constants for marketplace_share_info.
const (
	ShareTypePrivate = "private"
	ShareTypePublic  = "public"
)

// swagger:model ShareInfo
type ShareInfo struct {
	ID                        int       `gorm:"column:id;primaryKey"`
	OwnerID                   string    `gorm:"column:owner_id;size:64;not null;index:idx_marketplace_share_info_for_owner,priority:1" json:"owner_id,omitempty"`
	ClientID                  string    `gorm:"column:client_id;size:64;not null;index:idx_marketplace_share_info_for_client,priority:1" json:"client_id"`
	FilePathHash              string    `gorm:"column:file_path_hash;size:64;not null;index:idx_marketplace_share_info_for_owner,priority:2;index:idx_marketplace_share_info_for_client,priority:2" json:"file_path_hash,omitempty"`
	ShareType                 string    `gorm:"column:share_type;size:16;not null;default:private;index:idx_marketplace_share_info_for_owner,priority:3;index:idx_marketplace_share_info_for_client,priority:3" json:"share_type,omitempty"`
	ReEncryptionKey           string    `gorm:"column:re_encryption_key;not null" json:"re_encryption_key,omitempty"`
	ClientEncryptionPublicKey string    `gorm:"column:client_encryption_public_key;not null" json:"client_encryption_public_key,omitempty"`
	Revoked                   bool      `gorm:"column:revoked;not null" json:"revoked"`
	ExpiryAt                  time.Time `gorm:"column:expiry_at;not null" json:"expiry_at,omitempty"`
	AvailableAt               time.Time `gorm:"column:available_at;type:timestamp without time zone;not null;default:current_timestamp" json:"available_at,omitempty"`
}

func (ShareInfo) TableName() string {
	return "marketplace_share_info"
}

// add share if it already doesnot exist
func AddShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	if shareInfo.ShareType == "" {
		shareInfo.ShareType = ShareTypePrivate
	}
	return db.Model(&ShareInfo{}).Create(shareInfo).Error
}

// ListShareInfo returns list of files by a given clientID
func ListShareInfoClientID(ctx context.Context, ownerID string, limit common.Pagination) ([]ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var shares []ShareInfo
	query := db.Where("owner_id = ?", ownerID).Where("revoked = ?", false).Limit(limit.Limit).Offset(limit.Offset)

	err := query.Find(&shares).Error
	return shares, err
}

func DeleteShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	shareType := shareInfo.ShareType
	if shareType == "" {
		shareType = ShareTypePrivate
	}
	result := db.Model(&ShareInfo{}).
		Where("owner_id = ? AND client_id = ? AND file_path_hash = ? AND share_type = ? AND revoked = ?",
			shareInfo.OwnerID, shareInfo.ClientID, shareInfo.FilePathHash, shareType, false).
		Updates(map[string]interface{}{"revoked": true})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CheckPublicShareExists checks if any active public share exists for a file for a given owner.
func CheckPublicShareExists(ctx context.Context, ownerID, filePathHash string) (bool, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var count int64

	err := db.Model(&ShareInfo{}).
		Where("owner_id = ? AND file_path_hash = ? AND share_type = ? AND revoked = ?", ownerID, filePathHash, ShareTypePublic, false).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetPublicShareRecipients gets all active recipients of the public share for (owner, file).
func GetPublicShareRecipients(ctx context.Context, ownerID, filePathHash string) ([]ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var recipients []ShareInfo

	err := db.Model(&ShareInfo{}).
		Select("id", "owner_id", "client_id", "file_path_hash", "share_type", "expiry_at", "available_at", "revoked").
		Where("owner_id = ? AND file_path_hash = ? AND share_type = ? AND revoked = ?", ownerID, filePathHash, ShareTypePublic, false).
		Find(&recipients).Error

	return recipients, err
}

// RemovePublicShareRecipient revokes the public-share entry for (owner, file, recipient).
func RemovePublicShareRecipient(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)

	result := db.Model(&ShareInfo{}).
		Where("owner_id = ? AND client_id = ? AND file_path_hash = ? AND share_type = ? AND revoked = ?",
			shareInfo.OwnerID, shareInfo.ClientID, shareInfo.FilePathHash, ShareTypePublic, false).
		Updates(map[string]interface{}{"revoked": true})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeletePublicShareInfo revokes all public-share entries for (owner, file).
func DeletePublicShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)

	result := db.Model(&ShareInfo{}).
		Where("owner_id = ? AND file_path_hash = ? AND share_type = ? AND revoked = ?",
			shareInfo.OwnerID, shareInfo.FilePathHash, ShareTypePublic, false).
		Updates(map[string]interface{}{"revoked": true})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateShareInfo(ctx context.Context, shareInfo *ShareInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	shareType := shareInfo.ShareType
	if shareType == "" {
		shareType = ShareTypePrivate
	}
	return db.Model(&ShareInfo{}).
		Where("owner_id = ? AND client_id = ? AND file_path_hash = ? AND share_type = ?",
			shareInfo.OwnerID, shareInfo.ClientID, shareInfo.FilePathHash, shareType).
		Select("Revoked", "ReEncryptionKey", "ExpiryAt", "AvailableAt", "ClientEncryptionPublicKey").
		Updates(shareInfo).
		Error
}

// GetShareInfoByType returns the share row for (clientID, filePathHash, shareType) if any.
func GetShareInfoByType(ctx context.Context, clientID, filePathHash, shareType string) (*ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	if shareType == "" {
		shareType = ShareTypePrivate
	}
	shareInfo := &ShareInfo{}
	err := db.Model(&ShareInfo{}).
		Where("client_id = ? AND file_path_hash = ? AND share_type = ?", clientID, filePathHash, shareType).
		Take(shareInfo).Error

	if err != nil {
		return nil, err
	}
	return shareInfo, nil
}

// GetAnyActiveShare returns any non-revoked share for (clientID, filePathHash).
// Used for download: access is allowed if the user has any valid share (public or private).
func GetAnyActiveShare(ctx context.Context, clientID, filePathHash string) (*ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	shareInfo := &ShareInfo{}
	err := db.Model(&ShareInfo{}).
		Where("client_id = ? AND file_path_hash = ? AND revoked = ?", clientID, filePathHash, false).
		First(shareInfo).Error

	if err != nil {
		return nil, err
	}
	return shareInfo, nil
}

// GetShareInfo returns the first share row for (clientID, filePathHash). Prefer GetShareInfoByType or GetAnyActiveShare.
func GetShareInfo(ctx context.Context, clientID, filePathHash string) (*ShareInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	shareInfo := &ShareInfo{}
	err := db.Model(&ShareInfo{}).
		Where("client_id = ? AND file_path_hash = ?", clientID, filePathHash).
		Take(shareInfo).Error

	if err != nil {
		return nil, err
	}
	return shareInfo, nil
}
