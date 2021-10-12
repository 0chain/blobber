package allocation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	bconfig "github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	zencryption "github.com/0chain/gosdk/zboxcore/encryption"
	"github.com/0chain/gosdk/zcncore"
	mocket "github.com/selvatico/go-mocket"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type MockFileBlockGetter struct {
	filestore.IFileBlockGetter
}

var mockFileBlock []byte

func (MockFileBlockGetter) GetFileBlock(
	fsStore *filestore.FileFSStore,
	allocationID string,
	fileData *filestore.FileInputData,
	blockNum int64,
	numBlocks int64) ([]byte, error) {
	return []byte(mockFileBlock), nil
}

func setMockFileBlock(data []byte) {
	mockFileBlock = data
}

func resetMockFileBlock() {
	mockFileBlock = []byte("mock")
}

var encscheme zencryption.EncryptionScheme

func setupEncryptionScheme() {
	encscheme = zencryption.NewEncryptionScheme()
	mnemonic := client.GetClient().Mnemonic
	if _, err := encscheme.Initialize(mnemonic); err != nil {
		panic("initialize encscheme")
	}
	encscheme.InitForEncryption("filetype:audio")
}

func setup(t *testing.T) {
	// setup wallet
	w, err := zcncrypto.NewBLS0ChainScheme().GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	wBlob, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := zcncore.SetWalletInfo(string(wBlob), true); err != nil {
		t.Fatal(err)
	}

	// setup servers
	sharderServ := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
			},
		),
	)
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				n := zcncore.Network{Miners: []string{"miner 1"}, Sharders: []string{sharderServ.URL}}
				blob, err := json.Marshal(n)
				if err != nil {
					t.Fatal(err)
				}

				if _, err := w.Write(blob); err != nil {
					t.Fatal(err)
				}
			},
		),
	)

	if err := zcncore.InitZCNSDK(server.URL, "ed25519"); err != nil {
		t.Fatal(err)
	}
}

func init() {
	resetMockFileBlock()
	common.ConfigRateLimits()
	chain.SetServerChain(&chain.Chain{})
	config.Configuration.SignatureScheme = "bls0chain"
	logging.Logger = zap.NewNop()

	dir, _ := os.Getwd()
	if _, err := filestore.SetupFSStoreI(dir+"/tmp", MockFileBlockGetter{}); err != nil {
		panic(err)
	}
	bconfig.Configuration.MaxFileSize = int64(1 << 30)
}

func TestBlobberCore_RenameFile(t *testing.T) {
	setup(t)
	setupEncryptionScheme()

	sch := zcncrypto.NewBLS0ChainScheme()
	sch.Mnemonic = "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.GenerateKeys()
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
		newName         string
		allocRoot       string
		expectedMessage string
		expectingError  bool
		setupDbMock     func()
	}{
		{
			name:            "Filename_Change_Ok",
			allocChange:     &AllocationChange{},
			allocRoot:       "/",
			path:            "old_file.pdf",
			newName:         "new_file.pdf",
			expectedMessage: "some_new_file",
			expectingError:  false,
			setupDbMock: func() {
				mocket.Catcher.NewMock().OneTime().WithQuery(
					`SELECT * FROM "reference_objects" WHERE`,
				).WithReply(
					[]map[string]interface{}{{
						"id":          2,
						"level":       1,
						"lookup_hash": "lookup_hash",
					}},
				)

				// root ref
				mocket.Catcher.NewMock().OneTime().WithQuery(
					`SELECT * FROM "reference_objects" WHERE`,
				).WithArgs("", alloc.ID).WithReply(
					[]map[string]interface{}{{
						"id":          1,
						"level":       0,
						"lookup_hash": "lookup_hash_root",
					}},
				)

			},
		},
	}

	for _, tc := range testCases {
		datastore.MocketTheStore(t, false)
		tc.setupDbMock()

		ctx := context.TODO()
		db := datastore.GetStore().GetDB().Begin()
		ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)

		change := &RenameFileChange{AllocationID: alloc.ID, Path: tc.path, NewName: tc.newName}
		response, err := change.ProcessChange(ctx, tc.allocChange, tc.allocRoot)

		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}

		if response.Name != tc.expectedMessage {
			t.Fatal("failed!")
		}
	}
}

func makeTestAllocation(exp common.Timestamp) *Allocation {
	allocID := "allocation id"
	alloc := Allocation{
		Tx: "allocation tx",
		ID: allocID,
		Terms: []*Terms{
			{
				ID:           1,
				AllocationID: allocID,
			},
		},
		Expiration: exp,
	}
	return &alloc
}
