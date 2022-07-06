package allocation

import (
	"context"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	mocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func init() {
	logging.Logger = zap.NewNop()
}

func TestBlobberCore_UpdateFile(t *testing.T) {

	sch := zcncrypto.NewSignatureScheme("bls0chain")
	mnemonic := "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.RecoverKeys(mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = sch.GetPublicKey()
	alloc.OwnerID = client.GetClientID()

	testCases := []struct {
		name            string
		context         metadata.MD
		allocChange     *AllocationChange
		path            string
		filename        string
		allocRoot       string
		thumbnailHash   string
		hash            string
		allocationID    string
		expectedMessage string
		expectingError  bool
		setupDbMock     func()
	}{
		{
			name:           "Update thumbnail hash",
			allocChange:    &AllocationChange{},
			allocRoot:      "/",
			path:           "/test_file",
			filename:       "test_file",
			hash:           "content_hash",
			thumbnailHash:  "thumbnail_hash",
			allocationID:   alloc.ID,
			expectingError: false,
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":             1,
							"level":          0,
							"lookup_hash":    "lookup_hash_root",
							"path":           "/",
							"name":           "/",
							"allocation_id":  alloc.ID,
							"parent_path":    "",
							"content_hash":   "",
							"thumbnail_size": 00,
							"thumbnail_hash": "",
							"type":           reference.DIRECTORY,
							"created_at":     common.Now() - 3600,
						},
						{
							"id":             2,
							"level":          1,
							"lookup_hash":    "lookup_hash",
							"path":           "/test_file",
							"name":           "test_file",
							"content_hash":   "content_hash",
							"thumbnail_size": 300,
							"thumbnail_hash": "thumbnail_hash_old",
							"allocation_id":  alloc.ID,
							"parent_path":    "/",
							"type":           reference.FILE,
							"created_at":     common.Now() - 3600,
						},
					},
				)

				query = `UPDATE "reference_objects" SET`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"rows_affected": 1,
						},
					},
				)

			},
		},
		{
			name:           "Update content hash",
			allocChange:    &AllocationChange{},
			allocRoot:      "/",
			path:           "/test_file",
			filename:       "test_file",
			hash:           "content_hash",
			thumbnailHash:  "thumbnail_hash",
			allocationID:   alloc.ID,
			expectingError: false,
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":             1,
							"level":          0,
							"lookup_hash":    "lookup_hash_root",
							"path":           "/",
							"name":           "/",
							"allocation_id":  alloc.ID,
							"parent_path":    "",
							"content_hash":   "",
							"thumbnail_size": 00,
							"thumbnail_hash": "",
							"type":           reference.DIRECTORY,
							"created_at":     common.Now() - 3600,
						},
						{
							"id":             2,
							"level":          1,
							"lookup_hash":    "lookup_hash",
							"path":           "/test_file",
							"name":           "test_file",
							"content_hash":   "content_hash_old",
							"thumbnail_size": 300,
							"thumbnail_hash": "thumbnail_hash",
							"allocation_id":  alloc.ID,
							"parent_path":    "/",
							"type":           reference.FILE,
							"created_at":     common.Now() - 3600,
						},
					},
				)

				query = `UPDATE "reference_objects" SET`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"rows_affected": 1,
						},
					},
				)

			},
		},
		{
			name:           "Remove thumbnail",
			allocChange:    &AllocationChange{},
			allocRoot:      "/",
			path:           "/test_file",
			filename:       "test_file",
			hash:           "content_hash",
			thumbnailHash:  "",
			allocationID:   alloc.ID,
			expectingError: false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
				query := `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":             1,
							"level":          0,
							"lookup_hash":    "lookup_hash_root",
							"path":           "/",
							"name":           "/",
							"allocation_id":  alloc.ID,
							"parent_path":    "",
							"content_hash":   "",
							"thumbnail_size": 00,
							"thumbnail_hash": "",
							"type":           reference.DIRECTORY,
							"created_at":     common.Now() - 3600,
						},
						{
							"id":             2,
							"level":          1,
							"lookup_hash":    "lookup_hash",
							"path":           "/test_file",
							"name":           "test_file",
							"content_hash":   "content_hash",
							"thumbnail_size": 300,
							"thumbnail_hash": "thumbnail_hash",
							"allocation_id":  alloc.ID,
							"parent_path":    "/",
							"type":           reference.FILE,
							"created_at":     common.Now() - 3600,
						},
					},
				)

				query = `UPDATE "reference_objects" SET`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"rows_affected": 1,
						},
					},
				)

			},
		},
	}

	for _, tc := range testCases {
		fs := &MockFileStore{}
		if err := fs.Initialize(); err != nil {
			t.Fatal(err)
		}
		filestore.SetFileStore(fs)
		datastore.MocketTheStore(t, true)
		tc.setupDbMock()

		ctx := context.TODO()
		db := datastore.GetStore().GetDB().Begin()
		ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)

		change := &UpdateFileChanger{
			BaseFileChanger: BaseFileChanger{
				Path:                tc.path,
				Filename:            tc.filename,
				ActualSize:          2310,
				ActualThumbnailSize: 92,
				ActualThumbnailHash: tc.thumbnailHash,
				AllocationID:        tc.allocationID,
				Hash:                tc.hash,
				Size:                2310,
				ThumbnailHash:       tc.thumbnailHash,
				ThumbnailSize:       92,
				ChunkSize:           65536,
				IsFinal:             true,
			},
		}

		t.Run(tc.name, func(t *testing.T) {
			_, err := func() (*reference.Ref, error) {
				resp, err := change.ApplyChange(ctx, tc.allocChange, tc.allocRoot, common.Now()-1)
				if err != nil {
					return nil, err
				}

				err = change.CommitToFileStore(ctx)
				return resp, err
			}()

			if tc.expectingError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedMessage)
				return
			}
			require.Nil(t, err)
		})

	}
}
