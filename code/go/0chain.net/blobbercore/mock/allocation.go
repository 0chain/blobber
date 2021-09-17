package mock

import (
	"github.com/selvatico/go-mocket"
)

func MockGetAllocationByID(allocationID string, allocation map[string]interface{}) {
	gomocket.Catcher.NewMock().
		WithQuery(`SELECT * FROM "allocations" WHERE "allocations"."tx" = $1 ORDER BY "allocations"."id" LIMIT 1`).
		WithArgs(allocationID).
		WithReply([]map[string]interface{}{allocation})
}
