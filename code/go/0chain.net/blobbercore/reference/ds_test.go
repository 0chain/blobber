package reference

import (
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"fmt"
	"testing"
)

func TestMockDb(t *testing.T) {
	config.Configuration.DBHost = "localhost"
	config.Configuration.DBName = "blobber_meta"
	config.Configuration.DBPort = "5431"
	config.Configuration.DBUserName = "blobber_user"
	config.Configuration.DBPassword = ""

	datastore.GetStore().Open()
	db := datastore.GetStore().GetDB()
	ref := &Ref{}
	err := db.Where(&Ref{AllocationID: "4f928c7857fabb5737347c42204eea919a4777f893f35724f563b932f64e2367", Path: "/hack.txt"}).First(ref).Error
	if err != nil {
		fmt.Println("err", err)
		return
	}
	fmt.Println(string(ref.Attributes))
}
