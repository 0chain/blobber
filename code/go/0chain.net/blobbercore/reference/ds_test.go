package reference

import (
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"github.com/stretchr/testify/require"
	"testing"
)

// this is just a dummy snippet to connect to local database
func TestMockDb(t *testing.T) {
	config.Configuration.DBHost = "localhost"
	config.Configuration.DBName = "blobber_meta"
	config.Configuration.DBPort = "5431"
	config.Configuration.DBUserName = "blobber_user"
	config.Configuration.DBPassword = ""

	require.NoError(t, datastore.GetStore().Open())
	db := datastore.GetStore().GetDB()
	if db == nil {
		t.Log("err connecting to database")
		return
	}
	ref := &Ref{}
	err := db.Where(&Ref{AllocationID: "4f928c7857fabb5737347c42204eea919a4777f893f35724f563b932f64e2367", Path: "/hack.txt"}).
		First(ref).
		Error
	if err != nil {
		t.Log("err", err)
		return
	}
	t.Log(string(ref.Attributes))
}
