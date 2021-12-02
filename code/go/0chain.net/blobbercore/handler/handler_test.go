package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	zencryption "github.com/0chain/gosdk/zboxcore/encryption"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/0chain/gosdk/zboxcore/marker"
	"github.com/0chain/gosdk/zcncore"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	bconfig "github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
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

func setupHandlers() (*mux.Router, map[string]string) {
	router := mux.NewRouter()

	opPath := "/v1/file/objectpath/{allocation}"
	opName := "Object_Path"
	router.HandleFunc(opPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(ObjectPathHandler),
		),
	),
	).Name(opName)

	rpPath := "/v1/file/referencepath/{allocation}"
	rpName := "Reference_Path"
	router.HandleFunc(rpPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(ReferencePathHandler),
		),
	),
	).Name(rpName)

	sPath := "/v1/file/stats/{allocation}"
	sName := "Stats"
	router.HandleFunc(sPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(FileStatsHandler),
		),
	),
	).Name(sName)

	otPath := "/v1/file/objecttree/{allocation}"
	otName := "Object_Tree"
	router.HandleFunc(otPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(ObjectTreeHandler),
		),
	),
	).Name(otName)

	collPath := "/v1/file/collaborator/{allocation}"
	collName := "Collaborator"
	router.HandleFunc(collPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(CollaboratorHandler),
		),
	),
	).Name(collName)

	rPath := "/v1/file/rename/{allocation}"
	rName := "Rename"
	router.HandleFunc(rPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(RenameHandler),
		),
	),
	).Name(rName)

	cPath := "/v1/file/copy/{allocation}"
	cName := "Copy"
	router.HandleFunc(cPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(CopyHandler),
		),
	),
	).Name(cName)

	aPath := "/v1/file/attributes/{allocation}"
	aName := "Attributes"
	router.HandleFunc(aPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(UpdateAttributesHandler),
		),
	),
	).Name(aName)

	uPath := "/v1/file/upload/{allocation}"
	uName := "Upload"
	router.HandleFunc(uPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(UploadHandler),
		),
	),
	).Name(uName)

	dPath := "/v1/file/download/{allocation}"
	dName := "Download"
	router.HandleFunc(dPath, common.UserRateLimit(
		common.ToJSONResponse(
			WithConnection(DownloadHandler),
		),
	),
	).Name(dName)

	sharePath := "/v1/marketplace/shareinfo/{allocation}"
	shareName := "Share"
	router.HandleFunc(sharePath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(MarketPlaceShareInfoHandler),
		),
	),
	).Name(shareName)

	return router,
		map[string]string{
			opPath:    opName,
			rpPath:    rpName,
			sPath:     sName,
			otPath:    otName,
			collPath:  collName,
			rPath:     rName,
			cPath:     cName,
			aPath:     aName,
			uPath:     uName,
			sharePath: shareName,
			dPath:     dName,
		}
}

func isEndpointRequireSignature(name string) bool {
	switch name {
	case "Download":
		return false
	default:
		return true
	}
}

func isEndpointAllowGetReq(name string) bool {
	switch name {
	case "Stats", "Rename", "Copy", "Attributes", "Upload", "Share", "Download":
		return false
	default:
		return true
	}
}

func GetAuthTicketForEncryptedFile(allocationID string, remotePath string, fileHash string, clientID string, encPublicKey string) (string, error) {
	at := &marker.AuthTicket{}
	at.AllocationID = allocationID
	at.OwnerID = client.GetClientID()
	at.ClientID = clientID
	at.FileName = remotePath
	at.FilePathHash = fileHash
	if strings.HasSuffix(remotePath, "/") {
		at.RefType = fileref.DIRECTORY
	} else {
		at.RefType = fileref.FILE
	}
	timestamp := int64(common.Now())
	at.Expiration = timestamp + 7776000
	at.Timestamp = timestamp
	at.ReEncryptionKey = "regenkey"
	err := at.Sign()
	if err != nil {
		return "", err
	}
	atBytes, err := json.Marshal(at)
	if err != nil {
		return "", err
	}
	return string(atBytes), nil
}

func TestHandlers_Requiring_Signature(t *testing.T) {
	setup(t)

	clientJson := "{\"client_id\":\"2f34516ed8c567089b7b5572b12950db34a62a07e16770da14b15b170d0d60a9\",\"client_key\":\"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083\",\"keys\":[{\"public_key\":\"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083\",\"private_key\":\"9fef6ff5edc39a79c1d8e5eb7ca7e5ac14d34615ee49e6d8ca12ecec136f5907\"}],\"mnemonics\":\"expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe\",\"version\":\"1.0\",\"date_created\":\"2021-05-30 17:45:06.492093 +0545 +0545 m=+0.139083805\"}"
	require.NoError(t, client.PopulateClient(clientJson, "bls0chain"))
	setupEncryptionScheme()
	router, handlers := setupHandlers()

	sch := zcncrypto.NewSignatureScheme("bls0chain")
	//sch.Mnemonic = "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.RecoverKeys("expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = sch.GetPublicKey()
	alloc.OwnerID = client.GetClientID()

	const (
		path         = "/path"
		newName      = "new name"
		connectionID = "connection id"
	)

	type (
		args struct {
			w *httptest.ResponseRecorder
			r *http.Request
		}
		test struct {
			name        string
			args        args
			alloc       *allocation.Allocation
			setupDbMock func(mock sqlmock.Sqlmock)
			begin       func()
			end         func()
			wantCode    int
			wantBody    string
		}
	)
	negativeTests := make([]test, 0)
	for _, name := range handlers {
		if !isEndpointRequireSignature(name) {
			continue
		}
		baseSetupDbMock := func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
				WithArgs(alloc.Tx).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
						AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
				)

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
				WithArgs(alloc.ID).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "allocation_id"}).
						AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
				)

			mock.ExpectCommit()
		}

		emptySignature := test{
			name: name + "_Empty_Signature",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					url, err := router.Get(name).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					method := http.MethodGet
					if !isEndpointAllowGetReq(name) {
						method = http.MethodPost
					}
					r, err := http.NewRequest(method, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					return r
				}(),
			},
			alloc:       alloc,
			setupDbMock: baseSetupDbMock,
			wantCode:    http.StatusBadRequest,
			wantBody:    "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}\n\n",
		}
		negativeTests = append(negativeTests, emptySignature)

		wrongSignature := test{
			name: name + "_Wrong_Signature",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					url, err := router.Get(name).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					method := http.MethodGet
					if !isEndpointAllowGetReq(name) {
						method = http.MethodPost
					}
					r, err := http.NewRequest(method, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash("another data")
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc:       alloc,
			setupDbMock: baseSetupDbMock,
			wantCode:    http.StatusBadRequest,
			wantBody:    "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}\n\n",
		}
		negativeTests = append(negativeTests, wrongSignature)
	}

	positiveTests := []test{
		{
			name: "Object_Path_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/objectpath/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("block_num", "0")
					q.Set("path", path)
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, "/", reference.DIRECTORY, alloc.ID, "/").
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}).
							AddRow("/"),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Reference_Path_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/referencepath/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, path, alloc.ID, "/", "", alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}).
							AddRow("/"),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Stats_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/stats/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodPost, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				lookUpHash := reference.GetReferenceLookup(alloc.ID, path)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.FILE),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "file_stats"`)).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "write_markers"`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Object_Tree_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/objecttree/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, path, path+"/%", alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}).
							AddRow("/"),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Collaborator_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/collaborator/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					q.Set("collab_id", "collab id")
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				lookUpHash := reference.GetReferenceLookup(alloc.ID, path)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.FILE),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "collaborators"`)).
					WillReturnRows(
						sqlmock.NewRows([]string{"ref_id"}).
							AddRow(0),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Rename_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/rename/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					q.Set("new_name", newName)
					q.Set("connection_id", connectionID)
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodPost, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocation_connections" WHERE`)).
					WithArgs(connectionID, alloc.ID, alloc.OwnerID, allocation.DeletedConnection).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)

				lookUpHash := reference.GetReferenceLookup(alloc.ID, path)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.FILE),
					)

				aa := sqlmock.AnyArg()
				mock.ExpectExec(`INSERT INTO "allocation_connections"`).
					WithArgs(aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "allocation_changes"`)).
					WithArgs(aa, aa, aa, aa, aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Copy_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/copy/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					q.Set("new_name", newName)
					q.Set("connection_id", connectionID)
					q.Set("dest", "dest")
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodPost, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				aa := sqlmock.AnyArg()

				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocation_connections" WHERE`)).
					WithArgs(connectionID, alloc.ID, alloc.OwnerID, allocation.DeletedConnection).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)

				lookUpHash := reference.GetReferenceLookup(alloc.ID, path)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.FILE),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(aa, aa).
					WillReturnError(errors.New(""))

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.DIRECTORY),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.DIRECTORY),
					)

				mock.ExpectExec(`INSERT INTO "allocation_connections"`).
					WithArgs(aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "allocation_changes"`)).
					WithArgs(aa, aa, aa, aa, aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Attributes_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/attributes/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					q.Set("path", path)
					q.Set("new_name", newName)
					q.Set("connection_id", connectionID)

					attr := &reference.Attributes{}
					attrBytes, err := json.Marshal(attr)
					if err != nil {
						t.Fatal(err)
					}
					q.Set("attributes", string(attrBytes))
					url.RawQuery = q.Encode()

					r, err := http.NewRequest(http.MethodPost, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				aa := sqlmock.AnyArg()

				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocation_connections" WHERE`)).
					WithArgs(connectionID, alloc.ID, alloc.OwnerID, allocation.DeletedConnection).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)

				lookUpHash := reference.GetReferenceLookup(alloc.ID, path)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type"}).
							AddRow(reference.FILE),
					)

				mock.ExpectExec(`INSERT INTO "allocation_connections"`).
					WithArgs(aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "allocation_changes"`)).
					WithArgs(aa, aa, aa, aa, aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Upload_OK",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/upload/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					q := url.Query()
					formFieldByt, err := json.Marshal(&allocation.UpdateFileChanger{})
					if err != nil {
						t.Fatal(err)
					}
					q.Set("uploadMeta", string(formFieldByt))
					q.Set("path", path)
					q.Set("new_name", newName)
					q.Set("connection_id", connectionID)
					url.RawQuery = q.Encode()

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					root, _ := os.Getwd()
					file, err := os.Open(root + "/handler_test.go")
					if err != nil {
						t.Fatal(err)
					}
					fileField, err := formWriter.CreateFormFile("uploadFile", file.Name())
					if err != nil {
						t.Fatal(err)
					}
					fileB := make([]byte, 0)
					if _, err := io.ReadFull(file, fileB); err != nil {
						t.Fatal(err)
					}
					if _, err := fileField.Write(fileB); err != nil {
						t.Fatal(err)
					}
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				aa := sqlmock.AnyArg()

				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects"`)).
					WithArgs(aa).
					WillReturnError(gorm.ErrRecordNotFound)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocation_connections" WHERE`)).
					WithArgs(connectionID, alloc.ID, alloc.OwnerID, allocation.DeletedConnection).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)

				mock.ExpectExec(`INSERT INTO "allocation_connections"`).
					WithArgs(aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "allocation_changes"`)).
					WithArgs(aa, aa, aa, aa, aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
			},
			wantCode: http.StatusOK,
		},
		{
			name: "InsertShareInfo_OK_New_Share",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/marketplace/shareinfo/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					shareClientEncryptionPublicKey := "kkk"
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					require.NoError(t, formWriter.WriteField("encryption_public_key", shareClientEncryptionPublicKey))
					remotePath := "/file.txt"
					filePathHash := "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c"
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, filePathHash, shareClientID, sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_ticket", authTicket))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.Tx, "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "lookup_hash"}).
							AddRow("/file.txt", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c"),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs("da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(sqlmock.NewRows([]string{}))
				aa := sqlmock.AnyArg()

				mock.ExpectExec(`INSERT INTO "marketplace_share_info"`).
					WithArgs(client.GetClientID(), "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c", "regenkey", aa, false, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCode: http.StatusOK,
			wantBody: "{\"message\":\"Share info added successfully\"}\n",
		},
		{
			name: "UpdateShareInfo",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/marketplace/shareinfo/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					shareClientEncryptionPublicKey := "kkk"
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					require.NoError(t, formWriter.WriteField("encryption_public_key", shareClientEncryptionPublicKey))
					remotePath := "/file.txt"
					filePathHash := "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c"
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, filePathHash, shareClientID, sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_ticket", authTicket))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.Tx, "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "lookup_hash"}).
							AddRow("/file.txt", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c"),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs("da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id", "owner_id"}).
							AddRow("abcdefgh", "owner"),
					)
				aa := sqlmock.AnyArg()

				mock.ExpectExec(`UPDATE "marketplace_share_info"`).
					WithArgs("regenkey", "kkk", false, aa, "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantCode: http.StatusOK,
			wantBody: "{\"message\":\"Share info added successfully\"}\n",
		},
		{
			name: "RevokeShareInfo_OK_Existing_Share",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/marketplace/shareinfo/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					remotePath := "/file.txt"

					require.NoError(t, formWriter.WriteField("refereeClientID", shareClientID))
					require.NoError(t, formWriter.WriteField("path", remotePath))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodDelete, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.Tx, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "lookup_hash"}).
							AddRow("/file.txt", filePathHash),
					)

				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "marketplace_share_info"`)).
					WithArgs(true, "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", filePathHash).
					WillReturnResult(sqlmock.NewResult(0, 1))

			},
			wantCode: http.StatusOK,
			wantBody: "{\"message\":\"Path successfully removed from allocation\",\"status\":204}\n",
		},
		{
			name: "RevokeShareInfo_NotOK_For_Non_Existing_Share",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/marketplace/shareinfo/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					remotePath := "/file.txt"

					require.NoError(t, formWriter.WriteField("refereeClientID", shareClientID))
					require.NoError(t, formWriter.WriteField("path", remotePath))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodDelete, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.Tx, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "lookup_hash"}).
							AddRow("/file.txt", filePathHash),
					)

				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "marketplace_share_info"`)).
					WithArgs(true, "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", filePathHash).
					WillReturnResult(sqlmock.NewResult(0, 0))

			},
			wantCode: http.StatusOK,
			wantBody: "{\"message\":\"Path not found\",\"status\":404}\n",
		},
		{
			name: "DownloadFile_Record_Not_Found",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					remotePath := "/file.txt"

					require.NoError(t, formWriter.WriteField("path_hash", fileref.GetReferenceLookup(alloc.Tx, remotePath)))
					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).WillReturnError(gorm.ErrRecordNotFound)

			},
			wantCode: http.StatusBadRequest,
			wantBody: "{\"code\":\"download_file\",\"error\":\"download_file: invalid file path: record not found\"}\n\n",
		},
		{
			name: "DownloadFile_Unencrypted_return_file",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					remotePath := "/file.txt"

					require.NoError(t, formWriter.WriteField("path_hash", fileref.GetReferenceLookup(alloc.Tx, remotePath)))
					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "lookup_hash", "content_hash"}).
							AddRow("/file.txt", "f", filePathHash, "abcd"),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "collaborators" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnError(gorm.ErrRecordNotFound)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id"}).
							AddRow(client.GetClientID()),
					)

				aa := sqlmock.AnyArg()

				mock.ExpectExec(`(UPDATE "read_markers" SET)(.+)`).
					WithArgs(client.GetClientPublicKey(), alloc.ID, alloc.OwnerID, aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
			wantBody: "\"bW9jaw==\"\n", //base64encoded for mock string
		},
		{
			name: "DownloadFile_Encrypted_Permission_Denied_Unshared_File",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					remotePath := "/file.txt"

					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)
					require.NoError(t, formWriter.WriteField("path_hash", pathHash))
					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, pathHash, client.GetClientID(), sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}

					require.NoError(t, formWriter.WriteField("auth_token", authTicket))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "content_hash", "qCj3sXXeXUAByi1ERIbcfXzWN75dyocYzyRXnkStXio=", 65536),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "collaborators" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnError(gorm.ErrRecordNotFound)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id"}).
							AddRow(client.GetClientID()),
					)

				aa := sqlmock.AnyArg()

				mock.ExpectExec(`UPDATE "read_markers"`).
					WithArgs(client.GetClientPublicKey(), alloc.ID, alloc.OwnerID, aa, aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(client.GetClientID(), filePathHash).
					WillReturnError(gorm.ErrRecordNotFound)

				mock.ExpectCommit()
			},
			wantCode: http.StatusBadRequest,
			wantBody: "{\"error\":\"client does not have permission to download the file. share does not exist\"}\n\n",
		},
		{
			name: "DownloadFile_Encrypted_Permission_Allowed_shared_File",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)
					remotePath := "/file.txt"

					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)
					require.NoError(t, formWriter.WriteField("path_hash", pathHash))
					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, pathHash, client.GetClientID(), sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_token", authTicket))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := encscheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}
				header := make([]byte, 2*1024)
				copy(header[:], encMsg.MessageChecksum+","+encMsg.OverallChecksum)
				data := append(header, encMsg.EncryptedData...)
				setMockFileBlock(data)
			},
			end: func() {
				resetMockFileBlock()
			},
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "content_hash", encscheme.GetEncryptedKey(), 65536),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "collaborators" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnError(gorm.ErrRecordNotFound)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id"}).
							AddRow(client.GetClientID()),
					)

				aa := sqlmock.AnyArg()

				mock.ExpectExec(`UPDATE "read_markers"`).
					WithArgs(client.GetClientPublicKey(), alloc.ID, alloc.OwnerID, aa, aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				reEncryptionKey, _ := encscheme.GetReGenKey(encscheme.GetEncryptedKey(), "filetype:audio")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(client.GetClientID(), filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, encscheme.GetEncryptedKey()),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
			wantBody: "",
		},
		{
			name: "DownloadFile_Encrypted_InSharedFolder_Permission_Allowed_shared_File",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)

					remotePath := "/"
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)

					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
					require.NoError(t, formWriter.WriteField("path_hash", filePathHash))

					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, pathHash, client.GetClientID(), sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_token", authTicket))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := encscheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}
				header := make([]byte, 2*1024)
				copy(header[:], encMsg.MessageChecksum+","+encMsg.OverallChecksum)
				data := append(header, encMsg.EncryptedData...)
				setMockFileBlock(data)
			},
			end: func() {
				resetMockFileBlock()
			},
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "content_hash", encscheme.GetEncryptedKey(), "/", fileref.CHUNK_SIZE),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "collaborators" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnError(gorm.ErrRecordNotFound)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path"}).
							AddRow("/", "d", rootPathHash, rootPathHash, "content_hash", "", "."),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id"}).
							AddRow(client.GetClientID()),
					)

				aa := sqlmock.AnyArg()

				mock.ExpectExec(`UPDATE "read_markers"`).
					WithArgs(client.GetClientPublicKey(), alloc.ID, alloc.OwnerID, aa, aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				reEncryptionKey, _ := encscheme.GetReGenKey(encscheme.GetEncryptedKey(), "filetype:audio")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(client.GetClientID(), rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, encscheme.GetEncryptedKey()),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
			wantBody: "",
		},
		{
			name: "DownloadFile_Encrypted_InSharedFolderSubdirectory_Permission_Allowed_shared_File",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)

					remotePath := "/folder1"
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)

					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1/subfolder1/file.txt")
					require.NoError(t, formWriter.WriteField("path_hash", filePathHash))

					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, pathHash, client.GetClientID(), sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_token", authTicket))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := encscheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}
				header := make([]byte, 2*1024)
				copy(header[:], encMsg.MessageChecksum+","+encMsg.OverallChecksum)
				data := append(header, encMsg.EncryptedData...)
				setMockFileBlock(data)
			},
			end: func() {
				resetMockFileBlock()
			},
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1/subfolder1/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "content_hash", encscheme.GetEncryptedKey(), "/folder1/subfolder1", filestore.CHUNK_SIZE),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "collaborators" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnError(gorm.ErrRecordNotFound)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path"}).
							AddRow("/folder1", "d", rootPathHash, rootPathHash, "content_hash", "", "."),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
					WithArgs(client.GetClientID()).
					WillReturnRows(
						sqlmock.NewRows([]string{"client_id"}).
							AddRow(client.GetClientID()),
					)

				aa := sqlmock.AnyArg()

				mock.ExpectExec(`UPDATE "read_markers"`).
					WithArgs(client.GetClientPublicKey(), alloc.ID, alloc.OwnerID, aa, aa, aa, aa, aa, aa, aa, aa).
					WillReturnResult(sqlmock.NewResult(0, 0))

				reEncryptionKey, _ := encscheme.GetReGenKey(encscheme.GetEncryptedKey(), "filetype:audio")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(client.GetClientID(), rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, encscheme.GetEncryptedKey()),
					)

				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
			wantBody: "",
		},
		{
			name: "DownloadFile_Encrypted_InSharedFolder_WrongFilePath_Permission_Rejected_shared_File",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					body := bytes.NewBuffer(nil)
					formWriter := multipart.NewWriter(body)

					remotePath := "/folder1"
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)

					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder2/subfolder1/file.txt")
					require.NoError(t, formWriter.WriteField("path_hash", filePathHash))

					require.NoError(t, formWriter.WriteField("block_num", fmt.Sprintf("%d", 1)))
					authTicket, err := GetAuthTicketForEncryptedFile(alloc.ID, remotePath, pathHash, client.GetClientID(), sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_token", authTicket))
					rm := &marker.ReadMarker{}
					rm.ClientID = client.GetClientID()
					rm.ClientPublicKey = client.GetClientPublicKey()
					rm.BlobberID = ""
					rm.AllocationID = alloc.ID
					rm.ReadCounter = 1
					rm.OwnerID = client.GetClientID()
					err = rm.Sign()
					if err != nil {
						t.Fatal(err)
					}
					rmData, err := json.Marshal(rm)
					require.NoError(t, err)
					require.NoError(t, formWriter.WriteField("read_marker", string(rmData)))
					if err := formWriter.Close(); err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodPost, url.String(), body)
					r.Header.Add("Content-Type", formWriter.FormDataContentType())
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := encscheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}
				header := make([]byte, 2*1024)
				copy(header[:], encMsg.MessageChecksum+","+encMsg.OverallChecksum)
				data := append(header, encMsg.EncryptedData...)
				setMockFileBlock(data)
			},
			end: func() {
				resetMockFileBlock()
			},
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows(
							[]string{
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
							),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)

				filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder2/subfolder1/file.txt")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "content_hash", encscheme.GetEncryptedKey(), "/folder2/subfolder1", fileref.CHUNK_SIZE),
					)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "content_hash", "encrypted_key", "parent_path"}).
							AddRow("/folder1", "d", rootPathHash, rootPathHash, "content_hash", "", "/"),
					)

			},
			wantCode: http.StatusBadRequest,
			wantBody: "{\"code\":\"download_file\",\"error\":\"download_file: cannot verify auth ticket: invalid_parameters: Auth ticket is not valid for the resource being requested\"}\n\n",
		},
	}
	tests := append(positiveTests, negativeTests...)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := datastore.MockTheStore(t)
			test.setupDbMock(mock)

			if test.begin != nil {
				test.begin()
			}
			router.ServeHTTP(test.args.w, test.args.r)
			if test.end != nil {
				test.end()
			}

			assert.Equal(t, test.wantCode, test.args.w.Result().StatusCode)
			if test.wantCode != http.StatusOK || test.wantBody != "" {
				assert.Equal(t, test.wantBody, test.args.w.Body.String())
			}
		})
	}

	curDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(curDir + "/tmp"); err != nil {
		t.Fatal(err)
	}
}
