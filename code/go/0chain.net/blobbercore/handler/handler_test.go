package handler

import (
	"0chain.net/blobbercore/allocation"
	bconfig "0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/config"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zcncore"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"
)

func init() {
	common.ConfigRateLimits()
	chain.SetServerChain(&chain.Chain{})
	config.Configuration.SignatureScheme = "bls0chain"
	logging.Logger = zap.NewNop()

	dir, _ := os.Getwd()
	if _, err := filestore.SetupFSStore(dir + "/tmp"); err != nil {
		panic(err)
	}
	bconfig.Configuration.MaxFileSize = int64(1 << 30)
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

	marketplacePath := "/v1/marketplace/secret"
	mName := "MarketplaceInfo"
	router.HandleFunc(marketplacePath, common.UserRateLimit(
		common.ToJSONResponse(
			WithReadOnlyConnection(MarketPlaceSecretHandler),
		),
	),
	).Name(mName)

	return router,
		map[string]string{
			opPath:   opName,
			rpPath:   rpName,
			sPath:    sName,
			otPath:   otName,
			collPath: collName,
			rPath:    rName,
			cPath:    cName,
			aPath:    aName,
			uPath:    uName,
			marketplacePath: mName,
		}
}

func isEndpointAllowGetReq(name string) bool {
	switch name {
	case "Stats", "Rename", "Copy", "Attributes", "Upload":
		return false
	default:
		return true
	}
}

func TestMarketplaceApi(t *testing.T) {
	setup(t)
	router, handlers := setupHandlers()

	t.Run("marketplace_key_existing", func(t *testing.T) {
		mock := datastore.MockTheStore(t)
		setupDbMock := func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace" ORDER BY "marketplace"."mnemonic" LIMIT 1`)).
				WillReturnRows(
					sqlmock.NewRows([]string{"public_key", "private_key", "mnemonic"}).
						AddRow("pub", "prv", "a b c d"),
				)

			mock.ExpectCommit()
		}
		setupDbMock(mock)
		httprequest := func() *http.Request {
			handlerName := handlers["/v1/marketplace/secret"]

			url, err := router.Get(handlerName).URL()
			if err != nil {
				t.Fatal()
			}

			r, err := http.NewRequest(http.MethodGet, url.String(), nil)
			if err != nil {
				t.Fatal(err)
			}

			return r
		}()

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httprequest)
		assert.Equal(t, 200, 200)
		wantBody := `{"mnemonic":"a b c d"}` + "\n"
		assert.Equal(t, wantBody, recorder.Body.String())
	})

	t.Run("marketplace_create_new_key_and_return", func(t *testing.T) {
		mock := datastore.MockTheStore(t)
		setupDbMock := func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace" ORDER BY "marketplace"."mnemonic" LIMIT 1`)).
				WillReturnRows(
					sqlmock.NewRows([]string{"public_key", "private_key", "mnemonic"}),
				)

			mock.ExpectExec(`INSERT INTO "marketplace"`).
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(0, 0))


			mock.ExpectCommit()
		}
		setupDbMock(mock)
		httprequest := func() *http.Request {
			handlerName := handlers["/v1/marketplace/secret"]

			url, err := router.Get(handlerName).URL()
			if err != nil {
				t.Fatal()
			}

			r, err := http.NewRequest(http.MethodGet, url.String(), nil)
			if err != nil {
				t.Fatal(err)
			}

			return r
		}()

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httprequest)
		assert.Equal(t, 200, 200)
		marketplaceInfo := reference.MarketplaceInfo {}
		json.Unmarshal([]byte(recorder.Body.String()), &marketplaceInfo)
		assert.NotEmpty(t, marketplaceInfo)
		assert.Empty(t, marketplaceInfo.PublicKey)
		assert.Empty(t, marketplaceInfo.PrivateKey)
		assert.NotEmpty(t, marketplaceInfo.Mnemonic)
		fmt.Println(marketplaceInfo)
	})

}

func TestHandlers_Requiring_Signature(t *testing.T) {
	setup(t)

	router, handlers := setupHandlers()

	sch := zcncrypto.NewBLS0ChainScheme()
	_, err := sch.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = sch.GetPublicKey()
	alloc.OwnerID = sch.GetPublicKey()

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
			wantCode    int
			wantBody    string
		}
	)
	negativeTests := make([]test, 0)
	for _, name := range handlers {
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
					formFieldByt, err := json.Marshal(&allocation.UpdateFileChange{})
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

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocation_connections" WHERE`)).
					WithArgs(connectionID, alloc.ID, alloc.OwnerID, allocation.DeletedConnection).
					WillReturnRows(
						sqlmock.NewRows([]string{}).
							AddRow(),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects"`)).
					WithArgs(aa).
					WillReturnError(gorm.ErrRecordNotFound)

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
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := datastore.MockTheStore(t)
			test.setupDbMock(mock)

			router.ServeHTTP(test.args.w, test.args.r)

			assert.Equal(t, test.wantCode, test.args.w.Result().StatusCode)
			if test.wantCode != http.StatusOK {
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
