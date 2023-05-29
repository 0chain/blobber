//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/0chain/gosdk/zboxcore/marker"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

var mockFileBlock []byte

func setMockFileBlock(data []byte) {
	mockFileBlock = data
}

func resetMockFileBlock() {
	mockFileBlock = []byte("mock")
}

func signHash(client *client.Client, hash string) (string, error) {
	retSignature := ""
	for _, kv := range client.Keys {
		ss := zcncrypto.NewSignatureScheme("bls0chain")
		err := ss.SetPrivateKey(kv.PrivateKey)
		if err != nil {
			return "", err
		}
		if len(retSignature) == 0 {
			retSignature, err = ss.Sign(hash)
		} else {
			retSignature, err = ss.Add(retSignature, hash)
		}
		if err != nil {
			return "", err
		}
	}
	return retSignature, nil
}

func init() {
	resetMockFileBlock()
	chain.SetServerChain(&chain.Chain{})
	config.Configuration.SignatureScheme = "bls0chain"
	logging.Logger = zap.NewNop()
	ConfigRateLimits()

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	tDir := dir + "/tmp"

	if err := os.MkdirAll(tDir, 0777); err != nil {
		panic(err)
	}

	fs := &MockFileStore{}
	err = fs.Initialize()
	if err != nil {
		panic(err)
	}
	filestore.SetFileStore(fs)
}

func setupTestHandlers() (*mux.Router, map[string]string) {
	router := mux.NewRouter()

	sPath := "/v1/file/stats/{allocation}"
	sName := "Stats"
	router.HandleFunc(sPath,
		common.ToJSONResponse(
			WithReadOnlyConnection(FileStatsHandler))).Name(sName)

	rPath := "/v1/file/rename/{allocation}"
	rName := "Rename"
	router.HandleFunc(rPath,
		common.ToJSONResponse(
			WithReadOnlyConnection(RenameHandler))).Name(rName)

	cPath := "/v1/file/copy/{allocation}"
	cName := "Copy"
	router.HandleFunc(cPath,
		common.ToJSONResponse(
			WithReadOnlyConnection(CopyHandler))).Name(cName)

	uPath := "/v1/file/upload/{allocation}"
	uName := "Upload"
	router.HandleFunc(uPath,
		common.ToJSONResponse(
			WithReadOnlyConnection(UploadHandler))).Name(uName)

	return router,
		map[string]string{
			sPath: sName,
			rPath: rName,
			cPath: cName,
			uPath: uName,
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

func isEndpointUpload(name string) bool {
	return name == "Upload"
}

func isEndpointAllowGetReq(name string) bool {
	switch name {
	case "Stats", "Rename", "Copy", "Upload", "Download":
		return false
	default:
		return true
	}
}

func GetAuthTicketForEncryptedFile(ownerClient *client.Client, allocationID, remotePath, fileHash, clientID, encPublicKey string) (string, error) {
	at := &marker.AuthTicket{}
	at.AllocationID = allocationID
	at.OwnerID = ownerClient.ClientID
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
	at.Encrypted = true

	hash := encryption.Hash(at.GetHashData())
	var err error
	at.Signature, err = signHash(ownerClient, hash)
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

	clientJson := `{"client_id":"2f34516ed8c567089b7b5572b12950db34a62a07e16770da14b15b170d0d60a9","client_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","keys":[{"public_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","private_key":"9fef6ff5edc39a79c1d8e5eb7ca7e5ac14d34615ee49e6d8ca12ecec136f5907"}],"mnemonics":"expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe","version":"1.0","date_created":"2021-05-30 17:45:06.492093 +0545 +0545 m=+0.139083805"}`
	guestClientJson := `{"client_id":"213297e22c8282ff85d1d5c99f4967636fe68f842c1351b24bd497246cbd26d9","client_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","keys":[{"public_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","private_key":"19ca446f814dcd56e28e11d4147f73590a07c7f1a9a6012087808a8602024a08"}],"mnemonics":"crazy dutch object arrest jump fragile oak amateur taxi trigger gap aspect marriage hat slice wool island spike unlock alter include easily say ramp","version":"1.0","date_created":"2022-01-26T07:26:41+05:45"}`

	require.NoError(t, client.PopulateClients([]string{clientJson, guestClientJson}, "bls0chain"))
	clients := client.GetClients()

	ownerClient := clients[0]

	router, handlers := setupTestHandlers()

	sch := zcncrypto.NewSignatureScheme("bls0chain")
	//sch.Mnemonic = "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err := sch.RecoverKeys("expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe")
	if err != nil {
		t.Fatal(err)
	}

	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = ownerClient.Keys[0].PublicKey
	alloc.OwnerID = ownerClient.ClientID

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
	uploadNegativeTests := make([]test, 0)
	for _, name := range handlers {
		if !isEndpointRequireSignature(name) || !isEndpointUpload(name) {
			continue
		}

		baseSetupDbMock := func(mock sqlmock.Sqlmock) {
			aa := sqlmock.AnyArg()

			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
				WithArgs(alloc.Tx).
				WillReturnRows(
					sqlmock.NewRows(
						[]string{
							"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size", "file_options",
						},
					).
						AddRow(
							alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30), alloc.FileOptions,
						),
				)
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
				WithArgs(alloc.ID).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "allocation_id"}).
						AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
				)
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reference_objects"`)).
				WithArgs(aa, aa).
				WillReturnRows(
					sqlmock.NewRows([]string{"count"}).
						AddRow(0),
				)
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
					q := url.Query()
					formFieldByt, err := json.Marshal(
						&allocation.UploadFileChanger{
							BaseFileChanger: allocation.BaseFileChanger{Path: path}})
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

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set("Allocation-Id", alloc.ID)

					return r
				}(),
			},
			alloc:       alloc,
			setupDbMock: baseSetupDbMock,
			wantCode:    http.StatusBadRequest,
			wantBody:    "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}\n\n",
		}
		uploadNegativeTests = append(uploadNegativeTests, emptySignature)

		wrongSignature := test{
			name: name + "_Wrong_Signature",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					url, err := router.Get(name).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}
					q := url.Query()
					formFieldByt, err := json.Marshal(
						&allocation.UploadFileChanger{
							BaseFileChanger: allocation.BaseFileChanger{Path: path}})
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

					r.Header.Set("Content-Type", formWriter.FormDataContentType())
					hash := encryption.Hash("another data")
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set("Allocation-Id", alloc.ID)
					return r
				}(),
			},
			alloc:       alloc,
			setupDbMock: baseSetupDbMock,
			wantCode:    http.StatusBadRequest,
			wantBody:    "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}\n\n",
		}
		uploadNegativeTests = append(uploadNegativeTests, wrongSignature)
	}
	negativeTests := make([]test, 0)
	for _, name := range handlers {
		if !isEndpointRequireSignature(name) || isEndpointUpload(name) {
			continue
		}

		baseSetupDbMock := func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
				WithArgs(alloc.Tx).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id", "file_options"}).
						AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, alloc.FileOptions),
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
					r.Header.Set("Allocation-Id", alloc.ID)

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
					r.Header.Set("Allocation-Id", alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id", "file_options"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, alloc.FileOptions),
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
					r.Header.Set("Allocation-Id", alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.Tx).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id", "file_options"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, alloc.FileOptions),
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
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","name","path","hash","size","validation_root","fixed_merkle_root" FROM "reference_objects" WHERE`)).
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
					q.Set("dest", "/dest")
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
					r.Header.Set("Allocation-Id", alloc.ID)

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
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id", "file_options"}).
							AddRow(alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, alloc.FileOptions),
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
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","name","path","hash","size","validation_root","fixed_merkle_root" FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, lookUpHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"type", "name"}).
							AddRow(reference.FILE, "path"),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "path","type" FROM "reference_objects" WHERE`)).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type"}).
							AddRow("/dest", reference.DIRECTORY),
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
					formFieldByt, err := json.Marshal(
						&allocation.UploadFileChanger{
							BaseFileChanger: allocation.BaseFileChanger{Path: path}})
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
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)
					r.Header.Set("Allocation-Id", alloc.ID)
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
								"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size", "file_options",
							},
						).
							AddRow(
								alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30), alloc.FileOptions,
							),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reference_objects"`)).
					WithArgs(aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{"count"}).
							AddRow(0),
					)
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
	}
	tests := append(positiveTests, negativeTests...)
	tests = append(tests, uploadNegativeTests...)

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
