package allocation

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
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

func TestBlobberCore_FileChangerUpload(t *testing.T) {
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
		validationRoot      string
		fileIDMeta          map[string]string
		allocationID        string
		maxDirFilesPerAlloc int
		expectedMessage     string
		expectingError      bool
		setupDbMock         func()
	}{
		{
			name:                "Upload file changer success",
			allocChange:         &AllocationChange{Operation: constants.FileOperationInsert},
			validationRoot:      "new_validation_root",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectingError:      false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
			},
		},
		{
			name:                "Upload file changer fails when max dirs & files reached",
			allocChange:         &AllocationChange{},
			validationRoot:      "new_validation_root",
			allocationID:        alloc.ID,
			maxDirFilesPerAlloc: 5,
			expectedMessage:     "max_alloc_dir_files_reached: maximum files and directories already reached",
			expectingError:      true,
			setupDbMock: func() {
				mocket.Catcher.Reset()

				query := `SELECT count(*) FROM "reference_objects" WHERE allocation_id = $1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply([]map[string]interface{}{
					{"count": 5},
				})
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

			ctx := datastore.GetStore().CreateTransaction(context.TODO())

			fPath := "/new"
			hasher := filestore.GetNewCommitHasher(2310)
			pathHash := encryption.Hash(fPath)
			CreateConnectionChange("connection_id", pathHash, alloc)
			UpdateConnectionObjWithHasher("connection_id", pathHash, hasher)
			change := &UploadFileChanger{
				BaseFileChanger: BaseFileChanger{
					Filename:       filepath.Base(fPath),
					Path:           "/new",
					ActualSize:     2310,
					AllocationID:   tc.allocationID,
					ValidationRoot: tc.validationRoot,
					Size:           2310,
					ChunkSize:      65536,
					ConnectionID:   "connection_id",
				},
			}

			fileIDMeta := map[string]string{
				filepath.Dir(fPath): "fileID#1",
				fPath:               "fileID#2",
			}
			rootRef, _ := reference.GetReferencePathFromPaths(ctx, tc.allocationID, []string{change.Path}, []string{})
			err := func() error {
				_, err := change.ApplyChange(ctx, rootRef, tc.allocChange, "/", common.Now()-1, fileIDMeta)
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
