package allocation

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	"github.com/stretchr/testify/require"
)

func setUpTest() *reference.Ref {
	sch := zcncrypto.NewSignatureScheme("bls0chain")
	mnemonic := "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.RecoverKeys(mnemonic)
	if err != nil {
		return nil
	}
	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = sch.GetPublicKey()
	alloc.OwnerID = client.GetClientID()
	// ctx := context.TODO()
	fPath := "/child/abc.txt"
	change := &UploadFileChanger{
		BaseFileChanger: BaseFileChanger{
			Filename:       filepath.Base(fPath),
			Path:           fPath,
			ActualSize:     2310,
			AllocationID:   alloc.ID,
			ValidationRoot: "new_validation_root",
			Size:           2310,
			ChunkSize:      65536,
		},
	}

	fileIDMeta := map[string]string{
		filepath.Dir(fPath): "fileID#1",
		fPath:               "fileID#2",
	}

	rootRef := &reference.Ref{Type: reference.DIRECTORY, AllocationID: alloc.ID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}

	fields, err := common.GetPathFields(filepath.Dir(change.Path))
	if err != nil {
		return nil
	}

	dirRef := rootRef

	for i := 0; i < len(fields); i++ {
		// we need to find field[i] in dirRef.Children
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				found = true
				dirRef = child
			}
		}
		if !found {
			newDirRef := reference.NewDirectoryRef()
			newDirRef.AllocationID = alloc.ID
			newDirRef.Path = filepath.Join("/", strings.Join(fields[:i+1], "/"))
			newDirRef.ParentPath = filepath.Join("/", strings.Join(fields[:i], "/"))
			newDirRef.Name = fields[i]
			newDirRef.FileID = fileIDMeta[newDirRef.Path]

		}
	}

	return rootRef
}

func TestUploadMove(t *testing.T) {
	rootRef := setUpTest()
	require.NotNil(t, rootRef)
	// fmt.Println(rootRef)
	// hash := encryption.Hash("ba193448221dc56455d1e865ac49ef72b2551e273cecd48841a8e69164d4c661:6cabbaee069c0f5ae57eda46e0e90080c46ad930e7ff1e1feed16f619ff8ee25:e2d7de3b1cc9ced73fe8245ace4563e1f2c27463bec2c0743ce518cb0905c9ee")
	temp := []string{"fe7864ff75fb0eeb4c8a630443914219861515f2cc7f5c3e95b43a83590b8ec5"}
	allocID := "1"
	path := "dir1"
	newHash := encryption.Hash(fmt.Sprintf("%s:%s", allocID, path) + strings.Join(temp, ":"))
	// fmt.Println(hash)

	fmt.Println(newHash)

	// children := []string{"/bYQt0fEHrv_test.txt", "/Et3teomShn_test.txt", "/pNWes2zYGi_test.txt"}
	// // children := []string{"b", "a"}
	// res := strings.Compare(children[0], children[1])
	// fmt.Println(res)
	// sort.StringSlice(children).Sort()
	// fmt.Println(children)
	// addCollate := fmt.Sprint(`ALTER TABLE reference_objects ALTER COLUMN path TYPE varchar(1000) COLLATE "POSIX"`)
	// fmt.Println(addCollate)
}

func addChild(dirRef *reference.Ref, child *reference.Ref) {
	if dirRef.Children == nil {
		dirRef.Children = []*reference.Ref{}
	}

	// put it in a sorted order, already sorted so binary search can be used

}
