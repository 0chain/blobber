//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	zencryption "github.com/0chain/gosdk/zboxcore/encryption"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/0chain/gosdk/zboxcore/zboxutil"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

func setupDownloadHandlers() (*mux.Router, map[string]string) {
	router := mux.NewRouter()

	dPath := "/v1/file/download/{allocation}"
	dName := "Download"
	router.HandleFunc(dPath, common.ToJSONResponse(
		WithConnection(DownloadHandler))).Name(dName)

	return router, map[string]string{
		dPath: dName,
	}
}

func getEncryptionScheme(mnemonic string) (zencryption.EncryptionScheme, error) {
	encscheme := zencryption.NewEncryptionScheme()
	if _, err := encscheme.Initialize(mnemonic); err != nil {
		return nil, err
	}
	encscheme.InitForEncryption("filetype:audio")
	return encscheme, nil
}

func TestHandlers_Download(t *testing.T) {
	setup(t)

	clientJson := `{"client_id":"2f34516ed8c567089b7b5572b12950db34a62a07e16770da14b15b170d0d60a9","client_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","keys":[{"public_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","private_key":"9fef6ff5edc39a79c1d8e5eb7ca7e5ac14d34615ee49e6d8ca12ecec136f5907"}],"mnemonics":"expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe","version":"1.0","date_created":"2021-05-30 17:45:06.492093 +0545 +0545 m=+0.139083805"}`
	guestClientJson := `{"client_id":"213297e22c8282ff85d1d5c99f4967636fe68f842c1351b24bd497246cbd26d9","client_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","keys":[{"public_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","private_key":"19ca446f814dcd56e28e11d4147f73590a07c7f1a9a6012087808a8602024a08"}],"mnemonics":"crazy dutch object arrest jump fragile oak amateur taxi trigger gap aspect marriage hat slice wool island spike unlock alter include easily say ramp","version":"1.0","date_created":"2022-01-26T07:26:41+05:45"}`

	require.NoError(t, client.PopulateClients([]string{clientJson, guestClientJson}, "bls0chain"))
	clients := client.GetClients()

	ownerClient, guestClient := clients[0], clients[1]

	ownerScheme, err := getEncryptionScheme(ownerClient.Mnemonic)
	if err != nil {
		t.Fatal(err)
	}

	guestScheme, err := getEncryptionScheme(guestClient.Mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	// require.NoError(t, client.PopulateClient(clientJson, "bls0chain"))
	// setupEncryptionScheme()

	router, handlers := setupDownloadHandlers()

	sch := zcncrypto.NewSignatureScheme("bls0chain")
	//sch.Mnemonic = "expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe"
	_, err = sch.RecoverKeys("expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe")
	if err != nil {
		t.Fatal(err)
	}

	ts := time.Now().Add(time.Hour)
	alloc := makeTestAllocation(common.Timestamp(ts.Unix()))
	alloc.OwnerPublicKey = ownerClient.Keys[0].PublicKey
	alloc.OwnerID = ownerClient.ClientID

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
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

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
			name: "DownloadFile_Record_Not_Found",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					handlerName := handlers["/v1/file/download/{allocation}"]
					url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					remotePath := "/file.txt"
					connectionID := zboxutil.NewConnectionId()
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						require.NoError(t, err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path", remotePath)
					r.Header.Set("X-Path-Hash", fileref.GetReferenceLookup(alloc.Tx, remotePath))
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

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

					remotePath := "/file.txt"
					connectionID := zboxutil.NewConnectionId()
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}
					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path", remotePath)
					r.Header.Set("X-Path-Hash", fileref.GetReferenceLookup(alloc.Tx, remotePath))
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, alloc.OwnerID)
					r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

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
						sqlmock.NewRows([]string{"path", "type", "lookup_hash", "validation_root"}).
							AddRow("/file.txt", "f", filePathHash, "abcd"),
					)
				mock.ExpectCommit()
			},
			wantCode: http.StatusOK,
			wantBody: "bW9jaw==", //base64encoded for mock string
		},
		//TODO: redeeming is moved to a separate route
		// {
		// 	name: "DownloadFile_file_return_stale_readmarker",
		// 	args: args{
		// 		w: httptest.NewRecorder(),
		// 		r: func() *http.Request {
		// 			handlerName := handlers["/v1/file/download/{allocation}"]
		// 			url, err := router.Get(handlerName).URL("allocation", alloc.Tx)
		// 			if err != nil {
		// 				t.Fatal()
		// 			}

		// 			remotePath := "/file.txt"

		// 			rm := &marker.ReadMarker{}
		// 			rm.ClientID = ownerClient.ClientID
		// 			rm.ClientPublicKey = ownerClient.ClientKey
		// 			rm.BlobberID = node.Self.ID
		// 			rm.AllocationID = alloc.ID
		// 			rm.ReadCounter = 1
		// 			rm.SessionRC = 1
		// 			rm.OwnerID = ownerClient.ClientID
		// 			rm.Signature, err = signHash(ownerClient, rm.GetHash())
		// 			if err != nil {
		// 				t.Fatal(err)
		// 			}
		// 			rmData, err := json.Marshal(rm)
		// 			require.NoError(t, err)
		// 			r, err := http.NewRequest(http.MethodGet, url.String(), nil)
		// 			if err != nil {
		// 				t.Fatal(err)
		// 			}

		// 			hash := encryption.Hash(alloc.Tx)
		// 			sign, err := sch.Sign(hash)
		// 			if err != nil {
		// 				t.Fatal(err)
		// 			}

		// 			r.Header.Set("X-Path-Hash", fileref.GetReferenceLookup(alloc.Tx, remotePath))
		// 			r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
		// 			r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
		// 			r.Header.Set("X-Submit-RM", fmt.Sprint(true))
		// 			r.Header.Set("X-Read-Marker", string(rmData))
		// 			r.Header.Set(common.ClientSignatureHeader, sign)
		// 			r.Header.Set(common.ClientHeader, alloc.OwnerID)
		// 			r.Header.Set(common.ClientKeyHeader, alloc.OwnerPublicKey)
		// 			r.Header.Set(common.AllocationIdHeader, alloc.ID)

		// 			return r
		// 		}(),
		// 	},
		// 	alloc: alloc,
		// 	setupDbMock: func(mock sqlmock.Sqlmock) {
		// 		mock.ExpectBegin()

		// 		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
		// 			WithArgs(alloc.Tx).
		// 			WillReturnRows(
		// 				sqlmock.NewRows(
		// 					[]string{
		// 						"id", "tx", "expiration_date", "owner_public_key", "owner_id", "blobber_size",
		// 					},
		// 				).
		// 					AddRow(
		// 						alloc.ID, alloc.Tx, alloc.Expiration, alloc.OwnerPublicKey, alloc.OwnerID, int64(1<<30),
		// 					),
		// 			)

		// 		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
		// 			WithArgs(alloc.ID).
		// 			WillReturnRows(
		// 				sqlmock.NewRows([]string{"id", "allocation_id"}).
		// 					AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
		// 			)

		// 		filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
		// 		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "reference_objects" WHERE`)).
		// 			WithArgs(alloc.ID, filePathHash).
		// 			WillReturnRows(
		// 				sqlmock.NewRows([]string{"path", "type", "lookup_hash", "validation_root"}).
		// 					AddRow("/file.txt", "f", filePathHash, "abcd"),
		// 			)

		// 		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "read_markers" WHERE`)).
		// 			WithArgs(ownerClient.ClientID, alloc.ID).
		// 			WillReturnRows(
		// 				sqlmock.NewRows([]string{"client_id", "counter"}).
		// 					AddRow(ownerClient.ClientID, 23),
		// 			)

		// 		aa := sqlmock.AnyArg()

		// 		mock.ExpectExec(`UPDATE "read_markers"`).
		// 			WithArgs(aa, aa, aa, aa).
		// 			WillReturnResult(sqlmock.NewResult(0, 0))

		// 		mock.ExpectCommit()
		// 	},
		// 	wantCode: http.StatusBadRequest,
		// 	wantBody: "{\"code\":\"stale_read_marker\",\"error\":\"stale_read_marker: \"}\n\n",
		// },
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

					remotePath := "/file.txt"
					connectionID := zboxutil.NewConnectionId()
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, pathHash, guestClient.ClientID, ownerClient.Keys[0].PublicKey)
					if err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}
					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path-Hash", pathHash)
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Verify-Download", fmt.Sprint(false))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set("X-Auth-Token", authTicket)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, guestClient.ClientID)
					r.Header.Set(common.ClientKeyHeader, guestClient.ClientKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

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
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "validation_root", "qCj3sXXeXUAByi1ERIbcfXzWN75dyocYzyRXnkStXio=", 65536),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(guestClient.ClientID, filePathHash).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			wantCode: http.StatusBadRequest,
			wantBody: "{\"code\":\"invalid_share\",\"error\":\"invalid_share: client does not have permission to download the file. share does not exist\"}\n\n",
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

					remotePath := "/file.txt"
					connectionID := zboxutil.NewConnectionId()
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, pathHash, guestClient.ClientID, "")
					if err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}
					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}
					r.Header.Set("X-Path-Hash", pathHash)
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Verify-Download", fmt.Sprint(false))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set("X-Auth-Token", authTicket)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, guestClient.ClientID)
					r.Header.Set(common.ClientKeyHeader, guestClient.ClientKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := ownerScheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}

				header := make([]byte, EncryptionHeaderSize)
				copy(header, encMsg.MessageChecksum+encMsg.OverallChecksum)
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
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "validation_root", ownerScheme.GetEncryptedKey(), 65536),
					)

				guestPublicEncryptedKey, err := guestScheme.GetPublicKey()
				if err != nil {
					t.Fatal(err)
				}
				reEncryptionKey, err := ownerScheme.GetReGenKey(guestPublicEncryptedKey, "filetype:audio")

				if err != nil {
					t.Fatal(err)
				}

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(guestClient.ClientID, filePathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, guestPublicEncryptedKey),
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

					remotePath := "/"
					connectionID := zboxutil.NewConnectionId()
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)
					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/file.txt")
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, pathHash, guestClient.ClientID, "")
					if err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path-Hash", filePathHash)
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Verify-Download", fmt.Sprint(false))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set("X-Auth-Token", authTicket)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, guestClient.ClientID)
					r.Header.Set(common.ClientKeyHeader, guestClient.ClientKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := ownerScheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}

				header := make([]byte, EncryptionHeaderSize)
				copy(header, encMsg.MessageChecksum+encMsg.OverallChecksum)
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
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "validation_root", ownerScheme.GetEncryptedKey(), "/", fileref.CHUNK_SIZE),
					)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","path" FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path"}).
							AddRow("/", "d", rootPathHash, rootPathHash, "validation_root", "", "."),
					)

				gpbk, err := guestScheme.GetPublicKey()
				if err != nil {
					t.Fatal(err)
				}

				reEncryptionKey, _ := ownerScheme.GetReGenKey(gpbk, "filetype:audio")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(guestClient.ClientID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, gpbk),
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

					remotePath := "/folder1"
					connectionID := zboxutil.NewConnectionId()
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)

					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1/subfolder1/file.txt")
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, pathHash, guestClient.ClientID, "")
					if err != nil {
						t.Fatal(err)
					}

					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path-Hash", filePathHash)
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Verify-Download", fmt.Sprint(false))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set("X-Auth-Token", authTicket)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, guestClient.ClientID)
					r.Header.Set(common.ClientKeyHeader, guestClient.ClientKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := ownerScheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}

				header := make([]byte, EncryptionHeaderSize)
				copy(header, encMsg.MessageChecksum+encMsg.OverallChecksum)
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
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/folder1/subfolder1/file.txt", "f", filePathHash, filePathHash, "validation_root", ownerScheme.GetEncryptedKey(), "/folder1/subfolder1", filestore.CHUNK_SIZE),
					)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","path" FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path"}).
							AddRow("/folder1", "d", rootPathHash, rootPathHash, "validation_root", "", "."),
					)

				gpbk, err := guestScheme.GetPublicKey()
				if err != nil {
					t.Fatal(err)
				}

				reEncryptionKey, _ := ownerScheme.GetReGenKey(gpbk, "filetype:audio")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs(guestClient.ClientID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"re_encryption_key", "client_encryption_public_key"}).
							AddRow(reEncryptionKey, gpbk),
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

					remotePath := "/folder1"
					connectionID := zboxutil.NewConnectionId()
					pathHash := fileref.GetReferenceLookup(alloc.Tx, remotePath)

					filePathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder2/subfolder1/file.txt")
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, pathHash, guestClient.ClientID, "")
					if err != nil {
						t.Fatal(err)
					}
					r, err := http.NewRequest(http.MethodGet, url.String(), nil)
					if err != nil {
						t.Fatal(err)
					}

					hash := encryption.Hash(alloc.Tx)
					sign, err := sch.Sign(hash)
					if err != nil {
						t.Fatal(err)
					}

					r.Header.Set("X-Path-Hash", filePathHash)
					r.Header.Set("X-Block-Num", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Num-Blocks", fmt.Sprintf("%d", 1))
					r.Header.Set("X-Verify-Download", fmt.Sprint(false))
					r.Header.Set("X-Connection-ID", connectionID)
					r.Header.Set("X-Mode", DownloadContentFull)
					r.Header.Set("X-Auth-Token", authTicket)
					r.Header.Set(common.ClientSignatureHeader, sign)
					r.Header.Set(common.ClientHeader, guestClient.ClientID)
					r.Header.Set(common.ClientKeyHeader, guestClient.ClientKey)
					r.Header.Set(common.AllocationIdHeader, alloc.ID)

					return r
				}(),
			},
			alloc: alloc,
			begin: func() {
				dataToEncrypt := "data_to_encrypt"
				encMsg, err := ownerScheme.Encrypt([]byte(dataToEncrypt))
				if err != nil {
					t.Fatal(err)
				}

				header := make([]byte, EncryptionHeaderSize)
				copy(header, encMsg.MessageChecksum+encMsg.OverallChecksum)
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
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path", "chunk_size"}).
							AddRow("/file.txt", "f", filePathHash, filePathHash, "validation_root", ownerScheme.GetEncryptedKey(), "/folder2/subfolder1", fileref.CHUNK_SIZE),
					)

				rootPathHash := fileref.GetReferenceLookup(alloc.Tx, "/folder1")
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","path" FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, rootPathHash).
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "type", "path_hash", "lookup_hash", "validation_root", "encrypted_key", "parent_path"}).
							AddRow("/folder1", "d", rootPathHash, rootPathHash, "validation_root", "", "/"),
					)

			},
			wantCode: http.StatusBadRequest,
			wantBody: "{\"code\":\"invalid_authticket\",\"error\":\"invalid_authticket: cannot verify auth ticket: invalid_parameters: Auth ticket is not valid for the resource being requested\"}\n\n",
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
			data := test.args.w.Body.Bytes()
			m := make(map[string]interface{})
			err = json.Unmarshal(data, &m)
			require.NoError(t, err)

			if test.wantCode != http.StatusOK || test.wantBody != "" {
				fmt.Println("fprint", test.args.w.Body.String())
				var body string
				if m["Data"] != nil {
					body = m["Data"].(string)
					assert.Equal(t, test.wantBody, body)
				} else {
					assert.Equal(t, test.wantBody, test.args.w.Body.String())
				}
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
