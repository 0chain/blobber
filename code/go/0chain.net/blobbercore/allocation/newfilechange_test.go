package allocation

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/gosdk/constants"
	"github.com/stretchr/testify/assert"
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

func TestBlobberCore_NewFile(t *testing.T) {
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
		name                string
		context             metadata.MD
		allocChange         *AllocationChange
		hash                string
		allocationID        string
		maxDirFilesPerAlloc int
		expectedMessage     string
		expectingError      bool
		setupDbMock         func()
		expectedFiles       map[string]map[string]bool
	}{
		{
			name:                "New file success",
			allocChange:         &AllocationChange{Operation: constants.FileOperationInsert},
			hash:                "new_file_hash",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
			},
			expectedFiles: map[string]map[string]bool{
				alloc.ID: {
					"new_file_hash": true,
				},
			},
		},
		{
			name:                "New file fails when max dirs & files reached",
			allocChange:         &AllocationChange{},
			hash:                "new_file_hash",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectedMessage:     "max_alloc_dir_files_reached: maximum files and directories already reached",
			expectingError:      true,
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT count(*) FROM "reference_objects" WHERE allocation_id = $1 AND "reference_objects"."deleted_at" IS NULL`
				mocket.Catcher.NewMock().WithQuery(query).WithReply([]map[string]interface{}{
					{"count": 5},
				})
			},
			expectedFiles: map[string]map[string]bool{
				alloc.ID: {},
			},
		},
		{
			name:                "New directory fails when max dirs & files reached",
			allocChange:         &AllocationChange{},
			hash:                "new_dir_hash",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectedMessage:     "max_alloc_dir_files_reached: maximum files and directories already reached",
			expectingError:      true,
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT count(*) FROM "reference_objects" WHERE allocation_id =`
				mocket.Catcher.NewMock().WithQuery(query).WithReply([]map[string]interface{}{
					{"count": 5},
				})
			},
			expectedFiles: map[string]map[string]bool{
				alloc.ID: {},
			},
		},
	}

	for _, tt := range testCases {
		tc := tt

		t.Run(t.Name(), func(t *testing.T) {
			filesInit := map[string]map[string]bool{
				tc.allocationID: {},
			}

			datastore.MocketTheStore(t, true)
			filestore.UseMock(filesInit)
			tc.setupDbMock()

			config.Configuration.MaxAllocationDirFiles = tc.maxDirFilesPerAlloc

			ctx := context.TODO()
			db := datastore.GetStore().GetDB().Begin()
			ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)

			change := &NewFileChange{
				Filename:     "new",
				Path:         "/",
				ActualSize:   2310,
				Attributes:   reference.Attributes{WhoPaysForReads: common.WhoPaysOwner},
				AllocationID: tc.allocationID,
				Hash:         tc.hash,
				Size:         2310,
				ChunkSize:    65536,
			}

			err := func() error {
				_, err := change.ApplyChange(ctx, tc.allocChange, "/")
				if err != nil {
					return err
				}

				return change.CommitToFileStore(ctx)
			}()

			assert.Equal(t, tc.expectingError, err != nil)
			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedMessage)
			}

			require.EqualValues(t, tc.expectedFiles, filesInit)
		})
	}
}
