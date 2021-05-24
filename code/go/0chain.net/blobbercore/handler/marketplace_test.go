package handler

import (
	bconfig "0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/config"
	"0chain.net/core/logging"
	"encoding/json"
	"fmt"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zcncore"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
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
	bconfig.Configuration.PreEncryption.AutoGenerate = true
}

func setupTest(t *testing.T) {
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

func setupHandler() (*mux.Router, map[string]string) {
	router := mux.NewRouter()

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
			marketplacePath: mName,
		}
}

func TestMarketplaceApi(t *testing.T) {
	setupTest(t)
	router, handlers := setupHandler()

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

	t.Run("marketplace_key_existing_same_for_all_blobbers_read_from_config", func(t *testing.T) {
		datastore.MockTheStore(t)
		bconfig.Configuration.PreEncryption.AutoGenerate = false
		bconfig.Configuration.PreEncryption.Mnemonic =
			"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown"
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
		wantBody := `{"mnemonic":"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown"}` + "\n"
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
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
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
