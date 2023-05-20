package allocation

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	mocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
)

func TestMultiOp(t *testing.T) {
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
	datastore.MocketTheStore(t, true)
	ctx := context.TODO()
	db := datastore.GetStore().GetDB().Begin()
	ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)
	setupDbMock()
	fileIDMeta := make(map[string]string)
	fileIDMeta["/"] = randName()
	changes := uploadChanges(alloc.ID, "new_validation_root", fileIDMeta)

	config.Configuration.MaxAllocationDirFiles = 100
	rootRef := &reference.Ref{Type: reference.DIRECTORY, AllocationID: alloc.ID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}

	for _, change := range changes {
		_, err = change.ApplyChange(ctx, rootRef, nil, alloc.ID, common.Timestamp(ts.Unix()), fileIDMeta)
		require.Nil(t, err)
	}
	require.Equal(t, len(changes), len(rootRef.Children))

	diffChanges := make([]AllocationChangeProcessor, 0, len(changes))
	for i := 0; i < len(changes); i++ {
		switch i % 3 {
		case 0:
			deleteChange := getDeleteFileChange(alloc.ID, changes[i].Path, changes[i].Filename)
			diffChanges = append(diffChanges, deleteChange)
		case 1:
			updateChange := getUpdateChange(changes[i])
			diffChanges = append(diffChanges, updateChange)
		case 2:
			renameChange := getRenameChange(alloc.ID, changes[i].Path, changes[i].Filename)
			diffChanges = append(diffChanges, renameChange)
		}
	}
	fmt.Printf("rootRef %+v\n", rootRef)
	for ind, change := range diffChanges {
		_, err = change.ApplyChange(ctx, rootRef, nil, alloc.ID, common.Timestamp(ts.Unix()), nil)
		fmt.Println("index", ind, "children", len(rootRef.Children))
		require.Nil(t, err)
	}

	require.Equal(t, 6, len(rootRef.Children))

}

func uploadChanges(allocationID, validationRoot string, fileIDMeta map[string]string) []*UploadFileChanger {

	changes := make([]*UploadFileChanger, 0, 10)

	for i := 0; i < 10; i++ {
		newName := randName()
		change := &UploadFileChanger{
			BaseFileChanger: BaseFileChanger{
				Filename:       newName,
				Path:           filepath.Join("/", newName),
				ActualSize:     2310,
				AllocationID:   allocationID,
				ValidationRoot: validationRoot,
				Size:           2310,
				ChunkSize:      65536,
			},
		}
		fileIDMeta[filepath.Join("/", change.Filename)] = randName()
		changes = append(changes, change)
	}
	return changes
}

func getDeleteFileChange(allocID, path, filename string) *DeleteFileChange {
	return &DeleteFileChange{
		AllocationID: allocID,
		Path:         filepath.Join("/", filename),
	}
}

func getUpdateChange(uploadChanger *UploadFileChanger) *UpdateFileChanger {
	return &UpdateFileChanger{
		BaseFileChanger: BaseFileChanger{
			Filename:       uploadChanger.Filename,
			Path:           filepath.Join("/", uploadChanger.Filename),
			ActualSize:     uploadChanger.ActualSize,
			AllocationID:   uploadChanger.AllocationID,
			ValidationRoot: "updated_validation_root",
			Size:           uploadChanger.Size,
			ChunkSize:      uploadChanger.ChunkSize,
		},
	}
}

func getRenameChange(allocID, path, filename string) *RenameFileChange {
	return &RenameFileChange{
		AllocationID: allocID,
		Path:         filepath.Join("/", filename),
		NewName:      randName(),
	}
}

func randName() string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789")
	b := make([]rune, 10)

	for i := range b {
		ind, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		b[i] = letterRunes[ind.Int64()]
	}
	return string(b)
}

func setupDbMock() {
	mocket.Catcher.Reset()

	query := `SELECT count(*) FROM "reference_objects" WHERE allocation_id = $1`
	mocket.Catcher.NewMock().WithQuery(query).WithReply([]map[string]interface{}{
		{"count": 0},
	})
}
