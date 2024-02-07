package allocation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	mocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func init() {
	logging.Logger = zap.NewNop()
}

func TestBlobberCore_MoveFile(t *testing.T) {
	sch := zcncrypto.NewSignatureScheme("bls0chain")
	mnemonic := "expose culture dignity plastic digital couple promote best pool error" +
		" brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.RecoverKeys(mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = sch.GetPublicKey()
	alloc.OwnerID = client.GetClientID()

	testCases := []struct {
		name                string
		context             metadata.MD
		allocChange         *AllocationChange
		srcPath             string
		destination         string
		allocationID        string
		maxDirFilesPerAlloc int
		expectedMessage     string
		expectingError      bool
		fileIDMeta          map[string]string
		setupDbMock         func()
	}{
		{
			name:                "Move file success",
			srcPath:             "/orig.txt",
			destination:         "/",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
			fileIDMeta:          map[string]string{},
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           0,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           1,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)

				q2 := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 OR ("reference_objects"."allocation_id" = $2 AND "reference_objects"."parent_path" = $3) OR ("reference_objects"."allocation_id" = $4 AND "reference_objects"."parent_path" = $5) OR (parent_path = $6 AND allocation_id = $7)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path%`
				mocket.Catcher.NewMock().WithQuery(q2).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)

				query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           0,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           1,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)
			},
		},
		{
			name:                "Move file should not fail when max dirs & files reached",
			allocChange:         &AllocationChange{},
			srcPath:             "/orig.txt",
			destination:         "/target",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
			fileIDMeta:          map[string]string{"/target": "file_id"},
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)

				q2 := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR ("reference_objects"."allocation_id" = $5 AND "reference_objects"."parent_path" = $6) OR (parent_path = $7 AND allocation_id = $8)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().WithQuery(q2).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)
				query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           0,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_hash",
							"path":            "/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)
			},
		},
		{
			name:                "Move directory should succeed",
			allocChange:         &AllocationChange{},
			srcPath:             "/new",
			destination:         "/target",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
			fileIDMeta:          map[string]string{"/target": "file_id"},
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_dir_hash",
							"path":            "/new",
							"name":            "new",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              3,
							"level":           3,
							"lookup_hash":     "lookup_hash",
							"path":            "/new/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/new",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)

				q2 := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR ("reference_objects"."allocation_id" = $5 AND "reference_objects"."parent_path" = $6) OR (parent_path = $7 AND allocation_id = $8)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().WithQuery(q2).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_dir_hash",
							"path":            "/new",
							"name":            "new",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              3,
							"level":           3,
							"lookup_hash":     "lookup_hash",
							"path":            "/new/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/new",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)
				query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":              1,
							"level":           1,
							"lookup_hash":     "lookup_hash_root",
							"path":            "/",
							"name":            "/",
							"allocation_id":   alloc.ID,
							"parent_path":     "",
							"validation_root": "",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              2,
							"level":           2,
							"lookup_hash":     "lookup_dir_hash",
							"path":            "/new",
							"name":            "new",
							"allocation_id":   alloc.ID,
							"parent_path":     "/",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.DIRECTORY,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
						{
							"id":              3,
							"level":           3,
							"lookup_hash":     "lookup_hash",
							"path":            "/new/orig.txt",
							"name":            "orig.txt",
							"allocation_id":   alloc.ID,
							"parent_path":     "/new",
							"validation_root": "validation_root",
							"thumbnail_size":  00,
							"thumbnail_hash":  "",
							"type":            reference.FILE,
							"created_at":      common.Now() - 3600,
							"updated_at":      common.Now() - 1800,
						},
					},
				)
			},
		},
	}

	for _, tt := range testCases {
		tc := tt
		mut := &sync.Mutex{}
		t.Run(t.Name(), func(t *testing.T) {
			fs := &MockFileStore{}
			if err := fs.Initialize(); err != nil {
				t.Fatal(err)
			}
			filestore.SetFileStore(fs)
			datastore.MocketTheStore(t, true)
			tc.setupDbMock()

			config.Configuration.MaxAllocationDirFiles = tc.maxDirFilesPerAlloc
			config.Configuration.MaxObjectsInDir = 1000

			ctx := datastore.GetStore().CreateTransaction(context.TODO())

			change := &MoveFileChange{
				AllocationID: tc.allocationID,
				SrcPath:      tc.srcPath,
				DestPath:     tc.destination,
			}
			rootRef, err := reference.GetReferencePathFromPaths(ctx, tc.allocationID, []string{change.DestPath, change.SrcPath}, []string{change.SrcPath})
			require.Nil(t, err)
			err = func() error {
				_, err := change.ApplyChange(ctx, rootRef, tc.allocChange, "/", common.Now()-1, tc.fileIDMeta)
				if err != nil {
					return err
				}

				return change.CommitToFileStore(ctx, mut)
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
