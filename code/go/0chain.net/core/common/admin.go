package common

import (
	"net/http"

	"github.com/spf13/viper"
)

// global username and password used to access endpoints only by admin
var gUsername, gPassword string

func SetAdminCredentials() {
	gUsername = viper.GetString("admin.username")
	gPassword = viper.GetString("admin.password")
}

func AuthenticateAdmin(handler ReqRespHandlerf) ReqRespHandlerf {
	SetAdminCredentials()
	return func(w http.ResponseWriter, r *http.Request) {
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

		handler(w, r)
	}
}
