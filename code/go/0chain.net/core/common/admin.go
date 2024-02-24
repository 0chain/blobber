package common

import (
	"encoding/hex"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"go.uber.org/zap"
	"net/http"

	"github.com/spf13/viper"
)

// global username and password used to access endpoints only by admin
var gUsername, gPassword string
var isDevelopment bool
var PublicKey0box string

func SetAdminCredentials(devMode bool) {
	gUsername = viper.GetString("admin.username")
	gPassword = viper.GetString("admin.password")
	isDevelopment = devMode
}

func AuthenticateAdmin(handler ReqRespHandlerf) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isDevelopment {
			uname, passwd, ok := r.BasicAuth()
			if !ok {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("Admin only api")) // nolint
				return
			}

			if uname != gUsername || passwd != gPassword {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("Invalid username or password")) // nolint
				return
			}
		}

		handler(w, r)
	}
}

func Set0boxDetails() {
	logging.Logger.Info("Setting 0box details")
	PublicKey0box = viper.GetString("0box.public_key")
	logging.Logger.Info("0box public key", zap.Any("public_key", PublicKey0box))
}

func Authenticate0Box(handler ReqRespHandlerf) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {

		signature := r.Header.Get("Zbox-Signature")
		if signature == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
		if err := signatureScheme.SetPublicKey(PublicKey0box); err != nil {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		success, err := signatureScheme.Verify(signature, hex.EncodeToString([]byte(PublicKey0box)))
		if err != nil || !success {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid signature")) // nolint
			return
		}

		handler(w, r)
	}
}
