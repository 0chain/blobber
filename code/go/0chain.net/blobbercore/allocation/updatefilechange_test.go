package allocation

import (
	"context"
	"strings"
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
		name                 string
		context              metadata.MD
		allocChange          *AllocationChange
		path                 string
		filename             string
		allocRoot            string
		thumbnailHash        string
		hash                 string
		allocationID         string
		expectedMessage      string
		expectingError       bool
		setupDbMock          func()
		initDir, expectedDir map[string]map[string]bool
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

				query := `SELECT * FROM "reference_objects" WHERE`
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
			initDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash":       true,
					"thumbnail_hash_old": true,
				},
			},
			expectedDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash":   true,
					"thumbnail_hash": true,
				},
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

				query := `SELECT * FROM "reference_objects" WHERE`
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
			initDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash_old": true,
					"thumbnail_hash":   true,
				},
			},
			expectedDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash":   true,
					"thumbnail_hash": true,
				},
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

				query := `SELECT * FROM "reference_objects" WHERE`
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
			initDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash":   true,
					"thumbnail_hash": true,
				},
			},
			expectedDir: map[string]map[string]bool{
				alloc.ID: {
					"content_hash": true,
				},
			},
		},
	}

	for _, tc := range testCases {
		datastore.MocketTheStore(t, true)
		filestore.UseMock(tc.initDir)
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
				Attributes:          reference.Attributes{WhoPaysForReads: common.WhoPaysOwner},
				AllocationID:        tc.allocationID,
				Hash:                tc.hash,
				Size:                2310,
				ThumbnailHash:       tc.thumbnailHash,
				ThumbnailSize:       92,
				ChunkSize:           65536,
				IsFinal:             true,
			},
		}

		_, err := func() (*reference.Ref, error) {
			resp, err := change.ProcessChange(ctx, tc.allocChange, tc.allocRoot)
			if err != nil {
				return nil, err
			}

			err = change.CommitToFileStore(ctx)
			return resp, err
		}()

		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			if tc.expectingError && strings.Contains(tc.expectedMessage, err.Error()) {
				t.Fatal("expected error " + tc.expectedMessage)
				break
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		require.EqualValues(t, tc.expectedDir, tc.initDir)
	}
}
