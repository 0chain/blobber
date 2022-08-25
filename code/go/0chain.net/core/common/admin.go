package common

import (
	"net/http"

	"github.com/spf13/viper"
)

var username, password string

func SetAdminCredentials() {
	username = viper.GetString("admin.username")
	password = viper.GetString("admin.password")
}

func AuthenticateAdmin(handler ReqRespHandlerf) ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		uname, passwd, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Admin only api")) // nolint
			return
		}

		if uname != username || passwd != password {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid username or password")) // nolint
			return
		}

		handler(w, r)
	}
}
