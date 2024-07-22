package reference

import (
	"context"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// swagger:model PlaylistFile
type PlaylistFile struct {
	LookupHash string `gorm:"column:lookup_hash" json:"lookup_hash"`
	Name       string `gorm:"column:name" json:"name"`
	Path       string `gorm:"column:path" json:"path"`
	NumBlocks  int64  `gorm:"column:num_of_blocks" json:"num_of_blocks"`
	ParentPath string `gorm:"column:parent_path" json:"parent_path"`
	Size       int64  `gorm:"column:size;" json:"size"`
	MimeType   string `gorm:"column:mimetype" json:"mimetype"`
	Type       string `gorm:"column:type" json:"type"`
}

// LoadPlaylist load playlist
func LoadPlaylist(ctx context.Context, allocationID, path, since string) ([]PlaylistFile, error) {
	var files []PlaylistFile

	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)

		sinceId := 0

		if len(since) > 0 {
			tx.Raw("SELECT id FROM reference_objects WHERE allocation_id = ? and lookup_hash = ? ", allocationID, since).Row().Scan(&sinceId) //nolint: errcheck
		}

		db := tx.Table("reference_objects").
			Select([]string{"lookup_hash", "name", "path", "num_of_blocks", "parent_path", "size", "mimetype", "type"}).Order("id")
		if sinceId > 0 {
			db.Where("allocation_id = ? and parent_path = ? and type='f' and id > ? and name like '%.ts'", allocationID, path, sinceId)
		} else {
			db.Where("allocation_id = ? and parent_path = ? and type='f' and name like '%.ts'", allocationID, path)
		}

		return db.Find(&files).Error

	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func LoadPlaylistFile(ctx context.Context, allocationID, lookupHash string) (*PlaylistFile, error) {
	file := &PlaylistFile{}

	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		result := tx.Table("reference_objects").
			Select([]string{"lookup_hash", "name", "path", "num_of_blocks", "parent_path", "size", "mimetype", "type"}).
			Where("allocation_id = ? and lookup_hash = ?", allocationID, lookupHash).
			First(file)

		escapedLookupHash := sanitizeString(lookupHash)
		logging.Logger.Info("playlist", zap.String("allocation_id", allocationID), zap.String("lookup_hash", escapedLookupHash))
		return result.Error
	})

	if err != nil {
		return nil, err
	}

	return file, nil
}

func sanitizeString(input string) string {
	sanitized := strings.ReplaceAll(input, "\n", "")
	sanitized = strings.ReplaceAll(sanitized, "\r", "")
	return sanitized
}
