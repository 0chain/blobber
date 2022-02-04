package lock

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLock(t *testing.T) {
	max := 100

	for i := 0; i < max; i++ {
		lock1 := GetMutex("testlock", strconv.Itoa(i))

		lock1.Lock()

		require.Equal(t, 1, lock1.usedby)

		lock1.Unlock()

		require.Equal(t, 0, lock1.usedby)

		lock2 := GetMutex("testlock", strconv.Itoa(i))
		lock2.Lock()

		require.Equal(t, 1, lock2.usedby)

		lock2.Unlock()

		require.Equal(t, 0, lock2.usedby)
	}

	cleanUnusedMutexs()

	for i := 0; i < max; i++ {
		_, ok := lockPool["testlock:"+strconv.Itoa(i)]
		require.Equal(t, false, ok)
	}
}
