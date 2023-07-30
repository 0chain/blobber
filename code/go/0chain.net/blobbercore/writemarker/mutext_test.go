package writemarker

import (
	"context"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	gomocket "github.com/selvatico/go-mocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	logging.Logger = zap.NewNop()
}

func TestMutext_LockShouldWork(t *testing.T) {

	datastore.UseMocket(false)

	config.Configuration.WriteMarkerLockTimeout = 30 * time.Second

	m := &Mutex{
		ML: common.GetNewLocker(),
	}
	now := time.Now()

	tests := []struct {
		name         string
		allocationID string
		connectionID string
		requestTime  time.Time
		mock         func()
		assert       func(*testing.T, *LockResult, error)
	}{
		{
			name:         "Lock should work",
			allocationID: "lock_allocation_id",
			connectionID: "lock_connection_id",
			requestTime:  now,
			mock: func() {

			},
			assert: func(test *testing.T, r *LockResult, err error) {
				require.Nil(test, err)
				require.Equal(test, LockStatusOK, r.Status)
			},
		},
		{
			name:         "retry lock by same request should work if it is not timeout",
			allocationID: "lock_same_allocation_id",
			connectionID: "lock_same_connection_id",
			requestTime:  now,
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT * FROM "write_locks" WHERE allocation_id=$1 ORDER BY "write_locks"."allocation_id" LIMIT 1`).
					WithArgs("lock_same_allocation_id").
					WithReply([]map[string]interface{}{
						{
							"allocation_id": "lock_same_allocation_id",
							"connection_id": "lock_same_connection_id",
							"created_at":    now,
						},
					})
			},
			assert: func(test *testing.T, r *LockResult, err error) {
				require.Nil(test, err)
				require.Equal(test, LockStatusOK, r.Status)
				require.EqualValues(test, now.Unix(), r.CreatedAt)
			},
		},
		{
			name:         "lock should be pending if it already is locked by other session ",
			allocationID: "lock_allocation_id",
			connectionID: "lock_pending_connection_id",
			requestTime:  time.Now(),
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT * FROM "write_locks" WHERE allocation_id=$1 ORDER BY "write_locks"."allocation_id" LIMIT 1`).
					WithArgs("lock_allocation_id").
					WithReply([]map[string]interface{}{
						{
							"allocation_id": "lock_allocation_id",
							"connection_id": "lock_connection_id",
							"created_at":    time.Now().Add(-5 * time.Second),
						},
					})
			},
			assert: func(test *testing.T, r *LockResult, err error) {
				require.Nil(test, err)
				require.Equal(test, LockStatusPending, r.Status)
			},
		},
		{
			name:         "lock should be ok if it is timeout",
			allocationID: "lock_timeout_allocation_id",
			connectionID: "lock_timeout_2nd_connection_id",
			requestTime:  now,
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT * FROM "write_locks" WHERE allocation_id=$1 ORDER BY "write_locks"."allocation_id" LIMIT 1`).
					WithArgs("lock_timeout_allocation_id").
					WithReply([]map[string]interface{}{
						{
							"allocation_id": "lock_timeout_allocation_id",
							"connection_id": "lock_timeout_1st_connection_id",
							"created_at":    time.Now().Add(31 * time.Second),
						},
					})
			},
			assert: func(test *testing.T, r *LockResult, err error) {
				require.Nil(test, err)
				require.Equal(test, LockStatusPending, r.Status)
			},
		},
		{
			name:         "retry lock by same request should work if it is timeout",
			allocationID: "lock_same_timeout_allocation_id",
			connectionID: "lock_same_timeout_connection_id",
			requestTime:  now,
			mock: func() {
				gomocket.Catcher.NewMock().
					WithQuery(`SELECT * FROM "write_locks" WHERE allocation_id=$1 ORDER BY "write_locks"."allocation_id" LIMIT 1`).
					WithArgs("lock_same_timeout_allocation_id").
					WithReply([]map[string]interface{}{
						{
							"allocation_id": "lock_same_timeout_allocation_id",
							"connection_id": "lock_same_timeout_connection_id",
							"created_at":    now.Add(-config.Configuration.WriteMarkerLockTimeout),
						},
					})
			},
			assert: func(test *testing.T, r *LockResult, err error) {
				require.Nil(test, err)
				require.NotNil(test, r)
			},
		},
	}

	for _, it := range tests {

		t.Run(it.name,
			func(test *testing.T) {
				if it.mock != nil {
					it.mock()
				}
				r, err := m.Lock(context.TODO(), it.allocationID, it.connectionID)

				it.assert(test, r, err)

			},
		)

	}

}
