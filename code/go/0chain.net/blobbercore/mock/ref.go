package mock

import (
	gomocket "github.com/selvatico/go-mocket"
)

func MockRefNoExists(allocationTx, path string) {
	gomocket.Catcher.NewMock().
		WithQuery(`SELECT count(*) FROM "reference_objects" WHERE reference_objects.allocation_id = $1 and reference_objects.path = $2`).
		WithArgs(allocationTx, path).
		WithReply([]map[string]interface{}{
			{
				"count(*)": 0,
			},
		}).OneTime()

}

func MockRefExists(allocationTx, path string) {
	gomocket.Catcher.NewMock().
		WithQuery(`SELECT count(*) FROM "reference_objects" WHERE reference_objects.allocation_id = $1 and reference_objects.path = $2`).
		WithArgs(allocationTx, path).
		WithReply([]map[string]interface{}{
			{
				"count(*)": 1,
			},
		}).OneTime()

}
