package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zcncore"
)

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
