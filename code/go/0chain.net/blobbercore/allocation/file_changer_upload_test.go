package allocation

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/gosdk/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
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
		inodesMeta          *InodeMeta
		hash                string
		allocationID        string
		maxDirFilesPerAlloc int
		expectedMessage     string
		expectingError      bool
		setupDbMock         func()
	}{
		{
			name:                "Upload file changer success",
			allocChange:         &AllocationChange{Operation: constants.FileOperationInsert},
			hash:                "new_file_hash",
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
			hash:                "new_file_hash",
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

			fPath := "/new"
			change := &UploadFileChanger{
				BaseFileChanger: BaseFileChanger{
					Filename:     filepath.Base(fPath),
					Path:         "/new",
					ActualSize:   2310,
					AllocationID: tc.allocationID,
					Hash:         tc.hash,
					Size:         2310,
					ChunkSize:    65536,
				},
			}

			inodesMeta := func() *InodeMeta {
				fileID := int64(2)
				hash := encryption.Hash(strconv.FormatInt(fileID, 10))
				sign, _ := client.Sign(hash)
				in := Inode{
					AllocationID:   alloc.ID,
					LatestFileID:   fileID,
					OwnerSignature: sign,
				}

				inodesMeta := map[string]int64{
					filepath.Dir(fPath): 1,
					fPath:               2,
				}
				return &InodeMeta{MetaData: inodesMeta, LatestInode: in}
			}()
			err := func() error {
				_, err := change.ApplyChange(ctx, tc.allocChange, "/", common.Now()-1, inodesMeta)
				if err != nil {
					return err
				}

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
