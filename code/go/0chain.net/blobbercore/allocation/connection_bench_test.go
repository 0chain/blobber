package allocation

import (
	"fmt"
	"testing"

	"github.com/lithammer/shortuuid/v3"
)

var table = []struct {
	input int
}{
	{input: 100},
	{input: 1000},
	{input: 10000},
}

func BenchmarkConnectionObj(b *testing.B) {
	for _, v := range table {
		connectionIDs := make([]string, 0)
		for i := 0; i < v.input; i++ {
			connectionID := shortuuid.New()
			connectionIDs = append(connectionIDs, connectionID)
			UpdateConnectionObjSize(connectionID, int64(i))
		}

		b.Run(fmt.Sprintf("input_size_%d", v.input), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				connectionID := connectionIDs[i%v.input]
				GetConnectionObjSize(connectionID)
				UpdateConnectionObjSize(connectionID, int64(v.input))

				newConnectionID := shortuuid.New()
				go GetConnectionObjSize(newConnectionID)
				go UpdateConnectionObjSize(newConnectionID, int64(v.input))

			}
		})

	}
}
