package allocation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
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
			expectedMessage: "ref is not found",
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
				query := `SELECT count(*) FROM "reference_objects" WHERE (allocation_id=$1 AND path=$2) AND "reference_objects"."deleted_at" IS NULL`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply([]map[string]interface{}{
						{"count": 0},
					})

				query = `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply(
						[]map[string]interface{}{
							{
								"id":          1,
								"lookup_hash": "root_lookup_hash",
								"name":        "/",
								"path":        "/",
								"parent_path": "",
								"type":        "d",
								"level":       1,
								"created_at":  common.Now() - 3600,
							},
							{
								"id":          2,
								"lookup_hash": "lookup_hash",
								"name":        "old_file.pdf",
								"path":        "/old_file.pdf",
								"parent_path": "/",
								"type":        "f",
								"level":       42,
								"created_at":  common.Now() - 1800,
							},
						},
					)

				query = `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR (parent_path = $5 AND allocation_id = $6)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":          1,
							"level":       1,
							"lookup_hash": "lookup_hash_root",
							"path":        "/",
							"parent_path": "",
							"name":        "",
							"type":        "d",
							"created_at":  common.Now() - 3600,
						},
						{
							"id":          2,
							"level":       2,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir",
							"parent_path": "/",
							"name":        "old_dir",
							"type":        "d",
							"created_at":  common.Now() - 1800,
						},
						{
							"id":          3,
							"level":       3,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir/abc.def",
							"parent_path": "/old_dir",
							"name":        "abc.def",
							"type":        "f",
							"created_at":  common.Now() - 1800,
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

				query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":          1,
							"level":       1,
							"lookup_hash": "lookup_hash_root",
							"path":        "/",
							"parent_path": "",
							"name":        "",
							"type":        "d",
							"created_at":  common.Now() - 3600,
						},
						{
							"id":          2,
							"level":       2,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir",
							"parent_path": "/",
							"name":        "old_dir",
							"type":        "d",
							"created_at":  common.Now() - 1800,
						},
						{
							"id":          3,
							"level":       3,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir/abc.def",
							"parent_path": "/old_dir",
							"name":        "abc.def",
							"type":        "f",
							"created_at":  common.Now() - 1800,
						},
					},
				)
			},
		},
		{
			name:            "Filename_Change_Ok",
			allocChange:     &AllocationChange{},
			allocRoot:       "/",
			path:            "/old_file.pdf",
			newName:         "/new_file.pdf",
			expectedMessage: "",
			expectingError:  false,
			setupDbMock: func() {
				mocket.Catcher.Reset()
				query := `SELECT count(*) FROM "reference_objects" WHERE (allocation_id=$1 AND path=$2) AND "reference_objects"."deleted_at" IS NULL`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply([]map[string]interface{}{
						{"count": 0},
					})

				query = `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."path" = $2 OR (path LIKE $3 AND allocation_id = $4)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).
					WithReply(
						[]map[string]interface{}{
							{
								"id":          1,
								"lookup_hash": "root_lookup_hash",
								"name":        "/",
								"path":        "/",
								"parent_path": "",
								"type":        "d",
								"level":       1,
								"created_at":  common.Now() - 3600,
							},
							{
								"id":          2,
								"lookup_hash": "lookup_hash",
								"name":        "old_file.pdf",
								"path":        "/old_file.pdf",
								"parent_path": "/",
								"type":        "f",
								"level":       42,
								"created_at":  common.Now() - 1800,
							},
						},
					)

				query = `UPDATE "file_stats" SET`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"rows_affected": 1,
						},
					},
				)

				query = `SELECT * FROM "reference_objects" WHERE ("reference_objects"."allocation_id" = $1 AND "reference_objects"."parent_path" = $2 OR ("reference_objects"."allocation_id" = $3 AND "reference_objects"."parent_path" = $4) OR (parent_path = $5 AND allocation_id = $6)) AND "reference_objects"."deleted_at" IS NULL ORDER BY path`
				mocket.Catcher.NewMock().OneTime().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":          1,
							"level":       1,
							"lookup_hash": "lookup_hash_root",
							"path":        "/",
							"parent_path": "",
							"name":        "/",
							"type":        "d",
							"created_at":  common.Now() - 3600,
						},
						{
							"id":          2,
							"level":       1,
							"lookup_hash": "lookup_hash",
							"path":        "/old_file.pdf",
							"parent_path": "/",
							"name":        "old_file.pdf",
							"type":        "f",
							"created_at":  common.Now() - 3600,
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

				query = `SELECT * FROM "reference_objects" WHERE id = $1 AND "reference_objects"."deleted_at" IS NULL ORDER BY "reference_objects"."id" LIMIT 1`
				mocket.Catcher.NewMock().WithQuery(query).WithReply(
					[]map[string]interface{}{
						{
							"id":          1,
							"level":       1,
							"lookup_hash": "lookup_hash_root",
							"path":        "/",
							"parent_path": "",
							"name":        "",
							"type":        "d",
							"created_at":  common.Now() - 3600,
						},
						{
							"id":          2,
							"level":       2,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir",
							"parent_path": "/",
							"name":        "old_dir",
							"type":        "d",
							"created_at":  common.Now() - 1800,
						},
						{
							"id":          3,
							"level":       3,
							"lookup_hash": "lookup_hash",
							"path":        "/old_dir/abc.def",
							"parent_path": "/old_dir",
							"name":        "abc.def",
							"type":        "f",
							"created_at":  common.Now() - 1800,
						},
					},
				)
			},
		},
	}

	for _, tc := range testCases {
		datastore.MocketTheStore(t, true)
		tc.setupDbMock()

		ctx := context.TODO()
		db := datastore.GetStore().GetDB().Begin()
		ctx = context.WithValue(ctx, datastore.ContextKeyTransaction, db)
		t.Run(tc.name, func(t *testing.T) {
			change := &RenameFileChange{AllocationID: alloc.ID, Path: tc.path, NewName: tc.newName}
			rootRef, err := reference.GetReferencePathFromPaths(ctx, alloc.ID, []string{change.Path}, []string{})
			require.Nil(t, err)
			response, err := change.ApplyChange(ctx, rootRef, tc.allocChange, tc.allocRoot, common.Now()-1, nil)

			if tc.expectingError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedMessage)
				return
			}

			require.Nil(t, err)
			require.EqualValues(t, len(response.Children), 1)
			require.EqualValues(t, response.Children[0].Path, tc.newName)

		})

	}
}

func makeTestAllocation(exp common.Timestamp) *Allocation {
	allocID := "allocation_id"
	alloc := Allocation{
		Tx: "allocation_tx",
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
