package allocation

import (
	"context"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	mocket "github.com/selvatico/go-mocket"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func init() {
	logging.Logger = zap.NewNop()
}

func TestBlobberCore_CopyFile(t *testing.T) {
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
		fileIDMeta          map[string]string
		allocationID        string
		maxDirFilesPerAlloc int
		expectedMessage     string
		expectingError      bool
		setupDbMock         func()
	}{
		{
			name:        "Copy file success",
			allocChange: &AllocationChange{Operation: constants.FileOperationInsert},
			srcPath:     "/orig.txt",
			destination: "/",
			fileIDMeta: map[string]string{
				"/":         "fileID#1",
				"/orig.txt": "fileID#2",
			},
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
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

				q2 := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 OR ("reference_objects"."allocation_id" = $2 AND "reference_objects"."parent_path" = $3) OR ("reference_objects"."allocation_id" = $4 AND "reference_objects"."parent_path" = $5) OR (parent_path = $6 AND allocation_id = $7)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
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
		// {
		// 	name:         "Copy file fails when max dirs & files reached",
		// 	allocChange:  &AllocationChange{},
		// 	srcPath:      "/orig.txt",
		// 	destination:  "/target",
		// 	allocationID: alloc.ID,
		// 	fileIDMeta: map[string]string{
		// 		"/":                "fileID#1",
		// 		"/target":          "fileID#2",
		// 		"/target/orig.txt": "fileID#3",
		// 	},
		// 	maxDirFilesPerAlloc: 5,
		// 	expectedMessage:     "max_alloc_dir_files_reached: maximum files and directories already reached",
		// 	expectingError:      true,
		// 	setupDbMock: func() {
		// 		mocket.Catcher.Reset()

		// 		query := `SELECT count(*) FROM "reference_objects" WHERE allocation_id = $1`
		// 		mocket.Catcher.NewMock().WithQuery(query).WithReply([]map[string]interface{}{
		// 			{"count": 5},
		// 		})

		// 		query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
		// 		mocket.Catcher.NewMock().WithQuery(query).WithReply(
		// 			[]map[string]interface{}{
		// 				{
		// 					"id":              1,
		// 					"level":           0,
		// 					"lookup_hash":     "lookup_hash_root",
		// 					"path":            "/",
		// 					"name":            "/",
		// 					"allocation_id":   alloc.ID,
		// 					"parent_path":     "",
		// 					"validation_root": "",
		// 					"thumbnail_size":  00,
		// 					"thumbnail_hash":  "",
		// 					"type":            reference.DIRECTORY,
		// 					"created_at":      common.Now() - 3600,
		// 					"updated_at":      common.Now() - 1800,
		// 				},
		// 				{
		// 					"id":              2,
		// 					"level":           1,
		// 					"lookup_hash":     "lookup_hash",
		// 					"path":            "/orig.txt",
		// 					"name":            "orig.txt",
		// 					"allocation_id":   alloc.ID,
		// 					"parent_path":     "/",
		// 					"validation_root": "validation_root",
		// 					"thumbnail_size":  00,
		// 					"thumbnail_hash":  "",
		// 					"type":            reference.FILE,
		// 					"created_at":      common.Now() - 3600,
		// 					"updated_at":      common.Now() - 1800,
		// 				},
		// 			},
		// 		)
		// 	},
		// },
	}

	for _, tt := range testCases {
		tc := tt

		t.Run(t.Name(), func(t *testing.T) {
			fs := &MockFileStore{}
			if err := fs.Initialize(); err != nil {
				t.Fatal(err)
			}
			filestore.SetFileStore(fs)
			datastore.MocketTheStore(t, true)
			tc.setupDbMock()

			config.Configuration.MaxAllocationDirFiles = tc.maxDirFilesPerAlloc

			ctx := context.TODO()
			db := datastore.GetStore().GetDB().Begin()
			ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)

			change := &CopyFileChange{
				AllocationID: tc.allocationID,
				SrcPath:      tc.srcPath,
				DestPath:     tc.destination,
			}
			rootRef, err := reference.GetReferencePathFromPaths(ctx, alloc.ID, []string{change.DestPath, change.SrcPath}, []string{change.SrcPath})
			require.Nil(t, err)
			err = func() error {
				_, err := change.ApplyChange(ctx, rootRef, tc.allocChange, "/", common.Now()-1, tc.fileIDMeta)
				if err != nil {
					return err
				}
				require.Equal(t, 2, len(rootRef.Children))
				return change.CommitToFileStore(ctx)
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
