//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zboxcore/client"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

const (
	insertShare = "InsertShare"
	revokeShare = "RevokeShare"
	listShare   = "ListShare"
)

func setupShareHandlers() (*mux.Router, map[string]string) {
	router := mux.NewRouter()

	sharePath := "/v1/marketplace/shareinfo/{allocation}"

	router.HandleFunc(sharePath, common.ToJSONResponse(
		WithReadOnlyConnection(InsertShare))).Name(insertShare).Methods(http.MethodPost)

	router.HandleFunc(sharePath, common.ToJSONResponse(
		WithReadOnlyConnection(RevokeShare))).Name(revokeShare).Methods(http.MethodDelete)

	router.HandleFunc(sharePath, common.ToJSONResponse(
		WithReadOnlyConnection(ListShare))).Name(listShare).Methods(http.MethodGet)

	return router,
		map[string]string{
			insertShare: http.MethodPost,
			revokeShare: http.MethodDelete,
			listShare:   http.MethodGet,
		}
}

func TestHandlers_Share(t *testing.T) {
	setup(t)

	clientJson := `{"client_id":"2f34516ed8c567089b7b5572b12950db34a62a07e16770da14b15b170d0d60a9","client_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","keys":[{"public_key":"bc94452950dd733de3b4498afdab30ff72741beae0b82de12b80a14430018a09ba119ff0bfe69b2a872bded33d560b58c89e071cef6ec8388268d4c3e2865083","private_key":"9fef6ff5edc39a79c1d8e5eb7ca7e5ac14d34615ee49e6d8ca12ecec136f5907"}],"mnemonics":"expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe","version":"1.0","date_created":"2021-05-30 17:45:06.492093 +0545 +0545 m=+0.139083805"}`
	guestClientJson := `{"client_id":"213297e22c8282ff85d1d5c99f4967636fe68f842c1351b24bd497246cbd26d9","client_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","keys":[{"public_key":"7710b547897e0bddf93a28903875b244db4d320e4170172b19a5d51280c73522e9bb381b184fa3d24d6e1464882bf7f89d24ac4e8d05616d55eb857a6e235383","private_key":"19ca446f814dcd56e28e11d4147f73590a07c7f1a9a6012087808a8602024a08"}],"mnemonics":"crazy dutch object arrest jump fragile oak amateur taxi trigger gap aspect marriage hat slice wool island spike unlock alter include easily say ramp","version":"1.0","date_created":"2022-01-26T07:26:41+05:45"}`

	require.NoError(t, client.PopulateClients([]string{clientJson, guestClientJson}, "bls0chain"))
	clients := client.GetClients()

	ownerClient := clients[0]

	router, handlers := setupShareHandlers()

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
	for name, method := range handlers {
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
					fmt.Println(name)
					url, err := router.Get(name).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
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
			name: "InsertShareInfo_OK_New_Share",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					url, err := router.Get(insertShare).URL("allocation", alloc.Tx)
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
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, filePathHash, shareClientID, ownerClient.Keys[0].PublicKey)
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_ticket", authTicket))
					require.NoError(t, formWriter.WriteField("available_after", "0"))
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

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","path","lookup_hash","type","name" FROM "reference_objects" WHERE`)).
					WithArgs(alloc.Tx, "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(
						sqlmock.NewRows([]string{"path", "lookup_hash"}).
							AddRow("/file.txt", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c"),
					)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "marketplace_share_info" WHERE`)).
					WithArgs("da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
					WillReturnRows(sqlmock.NewRows([]string{}))
				aa := sqlmock.AnyArg()

				mock.ExpectQuery(`INSERT INTO "marketplace_share_info"`).
					WithArgs("2f34516ed8c567089b7b5572b12950db34a62a07e16770da14b15b170d0d60a9", "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c", "regenkey", aa, false, aa, aa).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
			},
			wantCode: http.StatusOK,
			wantBody: "{\"message\":\"Share info added successfully\"}\n",
		},
		{
			name: "UpdateShareInfo",
			args: args{
				w: httptest.NewRecorder(),
				r: func() *http.Request {
					url, err := router.Get(insertShare).URL("allocation", alloc.Tx)
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
					authTicket, err := GetAuthTicketForEncryptedFile(ownerClient, alloc.ID, remotePath, filePathHash, shareClientID, sch.GetPublicKey())
					if err != nil {
						t.Fatal(err)
					}
					require.NoError(t, formWriter.WriteField("auth_ticket", authTicket))
					require.NoError(t, formWriter.WriteField("available_after", "0"))
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

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","path","lookup_hash","type","name" FROM "reference_objects" WHERE`)).
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
					WithArgs("regenkey", "kkk", false, aa, aa, "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77", "f15383a1130bd2fae1e52a7a15c432269eeb7def555f1f8b9b9a28bd9611362c").
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
					u, err := router.Get(revokeShare).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					query := &url.Values{}
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					remotePath := "/file.txt"

					query.Add("refereeClientID", shareClientID)
					query.Add("path", remotePath)

					u.RawQuery = query.Encode()
					r, err := http.NewRequest(http.MethodDelete, u.String(), nil)

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
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","type" FROM "reference_objects" WHERE`)).
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
					u, err := router.Get(revokeShare).URL("allocation", alloc.Tx)
					if err != nil {
						t.Fatal()
					}

					query := &url.Values{}
					shareClientID := "da4b54d934890aa415bb043ce1126f2e30a96faf63a4c65c25bbddcb32824d77"
					remotePath := "/file.txt"

					query.Add("refereeClientID", shareClientID)
					query.Add("path", remotePath)

					u.RawQuery = query.Encode()
					r, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","type" FROM "reference_objects" WHERE`)).
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
