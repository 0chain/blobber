//go:build !integration
// +build !integration

package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"
)

func TestStorageHandler_StarOrUnstarRef(t *testing.T) {
	sch := zcncrypto.NewSignatureScheme("bls0chain")
	testWallet, _ := sch.RecoverKeys("expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe")

	alloc := makeTestAllocation(common.Timestamp(time.Now().Add(time.Hour).Unix()))
	alloc.OwnerPublicKey = testWallet.ClientKey
	alloc.OwnerID = testWallet.ClientID

	apiURL := "http://localhost/v1/file/star/" + alloc.ID
	validSign, _ := sch.Sign(encryption.Hash(alloc.Tx))

	for _, tc := range []struct {
		desc              string
		request           *http.Request
		requestHeaders    http.Header
		requestForm       url.Values
		wantResStatusCode int
		wantResBody       string
		setupDbMock       func(mock sqlmock.Sqlmock)
	}{
		// positive tests
		{
			desc:              "method is options",
			request:           mustHttpReq(http.MethodOptions, apiURL, nil),
			wantResStatusCode: 204,
		},
		{
			desc:              "success",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			requestForm:       formValues("starred", "true", "path_hash", "dummyHash"),
			wantResStatusCode: 200,
			wantResBody:       "{\"msg\":\"Updated ref star successfully\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
					WithArgs(alloc.ID, "dummyHash").
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}).
							AddRow("/"),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "reference_objects"`)).
					WillReturnRows(
						sqlmock.NewRows([]string{}),
					)
				mock.ExpectCommit()
			},
		},
		// negative tests
		{
			desc:              "method is not post",
			request:           mustHttpReq(http.MethodGet, apiURL, nil),
			wantResStatusCode: 405,
			wantResBody:       "",
		},
		{
			desc:              "expired allocation",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Invalid allocation id passed.verify_allocation: use of expired allocation\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, common.Timestamp(time.Now().Add(time.Hour*-1).Unix()), alloc.OwnerPublicKey, alloc.OwnerID),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)
			},
		},
		{
			desc:              "not the owner",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, "otherclient"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_operation\",\"error\":\"invalid_operation: Operation needs to be performed by the owner of the allocation\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "invalid signature",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, "badsignature"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "missing starred param",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			requestForm:       formValues(),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Missing starred.\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "invalid starred param",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			requestForm:       formValues("starred", "yes"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Invalid starred passed.\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "missing path and path_hash param",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			requestForm:       formValues("starred", "true"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Invalid path\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "pathHash is not valid",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			requestForm:       formValues("starred", "true", "path_hash", "dummyHash"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Invalid file path. record not found\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
					WithArgs(alloc.ID, "dummyHash").
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}),
					)
			},
		},
	} {
		tt := tc
		t.Run(tt.desc, func(t *testing.T) {
			mock := datastore.MockTheStore(t)
			if tt.setupDbMock != nil {
				tt.setupDbMock(mock)
			}

			router := mux.NewRouter()
			SetupHandlers(router)

			res := httptest.NewRecorder()
			tt.request.Header = tt.requestHeaders
			tt.request.Form = tt.requestForm

			router.ServeHTTP(res, tt.request)

			assert.Equal(t, tt.wantResStatusCode, res.Code)
			assert.Contains(t, res.Body.String(), tt.wantResBody)
		})
	}
}

func TestStorageHandler_ListStarredRefs(t *testing.T) {
	sch := zcncrypto.NewSignatureScheme("bls0chain")
	testWallet, _ := sch.RecoverKeys("expose culture dignity plastic digital couple promote best pool error brush upgrade correct art become lobster nature moment obtain trial multiply arch miss toe")

	alloc := makeTestAllocation(common.Timestamp(time.Now().Add(time.Hour).Unix()))
	alloc.OwnerPublicKey = testWallet.ClientKey
	alloc.OwnerID = testWallet.ClientID

	apiURL := "http://localhost/v1/file/starred/" + alloc.ID
	validSign, _ := sch.Sign(encryption.Hash(alloc.Tx))

	for _, tc := range []struct {
		desc              string
		request           *http.Request
		requestHeaders    http.Header
		wantResStatusCode int
		wantResBody       string
		setupDbMock       func(mock sqlmock.Sqlmock)
	}{
		// positive tests
		{
			desc:              "success",
			request:           mustHttpReq(http.MethodGet, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, validSign),
			wantResStatusCode: 200,
			wantResBody:       "{\"refs\":[{\"path\":\"/\",\"created_at\":\"0001-01-01T00:00:00Z\",\"updated_at\":\"0001-01-01T00:00:00Z\",\"chunk_size\":0},{\"path\":\"/file\",\"created_at\":\"0001-01-01T00:00:00Z\",\"updated_at\":\"0001-01-01T00:00:00Z\",\"chunk_size\":0}]}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
				mock.ExpectQuery(regexp.QuoteMeta(`FROM "reference_objects" WHERE`)).
					WithArgs(alloc.ID, true).
					WillReturnRows(
						sqlmock.NewRows([]string{"path"}).
							AddRow("/").
							AddRow("/file"),
					)
			},
		},
		// negative tests
		{
			desc:              "expired allocation",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_parameters\",\"error\":\"invalid_parameters: Invalid allocation id passed.verify_allocation: use of expired allocation\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "tx", "expiration_date", "owner_public_key", "owner_id"}).
							AddRow(alloc.ID, alloc.Tx, common.Timestamp(time.Now().Add(time.Hour*-1).Unix()), alloc.OwnerPublicKey, alloc.OwnerID),
					)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "terms" WHERE`)).
					WithArgs(alloc.ID).
					WillReturnRows(
						sqlmock.NewRows([]string{"id", "allocation_id"}).
							AddRow(alloc.Terms[0].ID, alloc.Terms[0].AllocationID),
					)
			},
		},
		{
			desc:              "not the owner",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, "otherclient"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_operation\",\"error\":\"invalid_operation: Operation needs to be performed by the owner of the allocation\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
		{
			desc:              "invalid signature",
			request:           mustHttpReq(http.MethodPost, apiURL, nil),
			requestHeaders:    headers(common.ClientHeader, alloc.OwnerID, common.ClientSignatureHeader, "badsignature"),
			wantResStatusCode: 400,
			wantResBody:       "{\"code\":\"invalid_signature\",\"error\":\"invalid_signature: Invalid signature\"}",
			setupDbMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations" WHERE`)).
					WithArgs(alloc.ID).
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
			},
		},
	} {
		tt := tc
		t.Run(tt.desc, func(t *testing.T) {
			mock := datastore.MockTheStore(t)
			if tt.setupDbMock != nil {
				tt.setupDbMock(mock)
			}

			router := mux.NewRouter()
			SetupHandlers(router)

			res := httptest.NewRecorder()
			tt.request.Header = tt.requestHeaders

			router.ServeHTTP(res, tt.request)

			assert.Equal(t, tt.wantResStatusCode, res.Code)
			assert.Contains(t, res.Body.String(), tt.wantResBody)
		})
	}
}

func headers(keyAndValues ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(keyAndValues); i += 2 {
		h.Add(keyAndValues[i], keyAndValues[i+1])
	}
	return h
}

func formValues(keyAndValues ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(keyAndValues); i += 2 {
		v.Add(keyAndValues[i], keyAndValues[i+1])
	}
	return v
}

func mustHttpReq(method string, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	return req
}
