package reference

import (
	"context"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/stretchr/testify/require"
)

// this is just a dummy snippet to connect to local database
func TestMockDb(t *testing.T) {
	t.Skip("Fails as the data store is not mocked, so Open returns a dial error")
	config.Configuration.DBHost = "localhost"
	config.Configuration.DBName = "blobber_meta"
	config.Configuration.DBPort = "5431"
	config.Configuration.DBUserName = "blobber_user"
	config.Configuration.DBPassword = ""

	require.NoError(t, datastore.GetStore().Open())
	_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		ref := &Ref{}
		err := tx.Where(&Ref{AllocationID: "4f928c7857fabb5737347c42204eea919a4777f893f35724f563b932f64e2367", Path: "/hack.txt"}).
			First(ref).
			Error
		require.NoError(t, err)

		return nil
	})
}
