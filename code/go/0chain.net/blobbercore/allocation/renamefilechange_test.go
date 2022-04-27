package allocation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

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
	"github.com/DATA-DOG/go-sqlmock"
	mocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

var mockFileBlock []byte

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
	w, err := zcncrypto.NewSignatureScheme("bls0chain").GenerateKeys()
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
func setupMockForFileManagerInit(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations"`)).
		WillReturnRows(sqlmock.NewRows(
			[]string{
				"id", "blobber_size", "blobber_size_used",
			},
		).AddRow(
			"allocation_id", 655360000, 6553600,
		),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reference_objects" WHERE`)).
		WillReturnRows(
			sqlmock.NewRows([]string{"count"}).AddRow(1000),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT sum(size) as file_size FROM "reference_objects" WHERE`)).
		WillReturnRows(
			sqlmock.NewRows([]string{"file_size"}).AddRow(6553600),
		)

}

func init() {
	resetMockFileBlock()
	common.ConfigRateLimits()
	chain.SetServerChain(&chain.Chain{})
	config.Configuration.SignatureScheme = "bls0chain"
	logging.Logger = zap.NewNop()

	mock := datastore.MockTheStore(nil)
	setupMockForFileManagerInit(mock)

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	tDir := dir + "/tmp"
	if err := os.MkdirAll(tDir, 0777); err != nil {
		panic(err)
	}

	fs := &filestore.MockStore{}
	err = fs.Initialize()
	if err != nil {
		panic(err)
	}
	filestore.SetFileStore(fs)
}

func TestBlobberCore_RenameFile(t *testing.T) {
	setup(t)
	setupEncryptionScheme()

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
		newName         string
		allocRoot       string
		expectedMessage string
		expectingError  bool
		setupDbMock     func()
	}{
		{
			name:            "Cant_find_file_object",
			allocChange:     &AllocationChange{},
			allocRoot:       "/",
			path:            "/old_dir",
			newName:         "/new_dir",
			expectedMessage: "Invalid path. Could not find object tree",
			expectingError:  true,
			setupDbMock: func() {
				mocket.Catcher.Reset()
			},
		},
		{
			name:            "Dirname_Change_Ok",
			allocChange:     &AllocationChange{},
			allocRoot:       "/",
			path:            "/old_dir",
			newName:         "/new_dir",
			expectedMessage: "",
			expectingError:  false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
				query := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path%!!(string=allocation id)!(string=/old_dir/%!)(MISSING)!(string=/old_dir)(EXTRA string=allocation id)`
				mocket.Catcher.NewMock().OneTime().WithQuery(
					`SELECT * FROM "reference_objects" WHERE`,
				).WithQuery(query).
					WithReply(
						[]map[string]interface{}{{
							"id":          2,
							"level":       1,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir",
						}},
					)
				query = `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","attributes","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR (parent_path = $5 AND allocation_id = $6)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path%!!(string=allocation id)!(string=)!(string=/)!(string=allocation id)!(string=/old_dir)(EXTRA string=allocation id)`
				mocket.Catcher.NewMock().OneTime().WithQuery(
					`SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","attributes","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE`,
				).WithQuery(query).WithReply(
					[]map[string]interface{}{{
						"id":          1,
						"level":       0,
						"lookup_hash": "lookup_hash_root",
						"path":        "/",
						"parent_path": ".",
					},
						{
							"id":          2,
							"level":       1,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir",
							"parent_path": "/",
						}},
				)
				mocket.Catcher.NewMock().WithQuery(`INSERT INTO "reference_objects"`).
					WithID(1)
			},
		},
		{
			name:            "Filename_Change_Ok",
			allocChange:     &AllocationChange{},
			allocRoot:       "/",
			path:            "old_file.pdf",
			newName:         "new_file.pdf",
			expectedMessage: "",
			expectingError:  false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
				query := `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path%!!(string=allocation id)!(string=old_file.pdf/%!)(MISSING)!(string=old_file.pdf)(EXTRA string=allocation id)`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply(
						[]map[string]interface{}{{
							"id":          2,
							"level":       1,
							"lookup_hash": "lookup_hash",
							"path":        "old_file.pdf",
						}},
					)
				query = `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","attributes","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR (parent_path = $5 AND allocation_id = $6)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path%!!(string=allocation id)!(string=)!(string=.)!(string=allocation id)!(string=old_file.pdf)(EXTRA string=allocation id)`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).WithReply(
					[]map[string]interface{}{{
						"id":          1,
						"level":       0,
						"lookup_hash": "lookup_hash_root",
						"path":        "/",
						"parent_path": ".",
					},
						{
							"id":          2,
							"level":       1,
							"lookup_hash": "lookup_hash",
							"path":        "old_file.pdf",
							"parent_path": "/",
						}},
				)
				query = `SELECT "id","allocation_id","type","name","path","parent_path","size","hash","path_hash","content_hash","merkle_root","actual_file_size","actual_file_hash","attributes","chunk_size","lookup_hash","thumbnail_hash","write_marker","level" FROM "reference_objects" WHERE "id" = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1%!(EXTRA int64=1)`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply(
						[]map[string]interface{}{{
							"id":          1,
							"level":       0,
							"lookup_hash": "lookup_hash_root",
							"path":        "/",
							"parent_path": ".",
						}},
					)
				mocket.Catcher.NewMock().WithQuery(`INSERT INTO "reference_objects"`).
					WithID(1)
			},
		},
	}

	for _, tc := range testCases {
		datastore.MocketTheStore(t, true)
		tc.setupDbMock()

		ctx := context.TODO()
		db := datastore.GetStore().GetDB().Begin()
		ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)
		change := &RenameFileChange{AllocationID: alloc.ID, Path: tc.path, NewName: tc.newName}
		response, err := change.ApplyChange(ctx, tc.allocChange, tc.allocRoot)
		if err != nil {
			if !tc.expectingError {
				t.Fatal(err)
			}

			if tc.expectingError && strings.Contains(tc.expectedMessage, err.Error()) {
				t.Fatal("expected error " + tc.expectedMessage)
			}

			continue
		}

		if tc.expectingError {
			t.Fatal("expected error")
		}
		require.EqualValues(t, len(response.Children), 1)
		require.EqualValues(t, response.Children[0].Path, tc.newName)
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
