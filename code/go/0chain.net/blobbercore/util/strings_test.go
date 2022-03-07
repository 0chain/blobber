package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitPath(t *testing.T) {

	tests := []struct {
		name  string
		path  string
		items []string
	}{
		{
			name:  "empty root",
			path:  "/",
			items: []string{},
		},
		{
			name:  "only 1 directory level deep",
			path:  "/file1",
			items: []string{"file1"},
		},
		{
			name:  "nested directories",
			path:  "/dir1/dir2/dir3/file1",
			items: []string{"dir1", "dir2", "dir3", "file1"},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(t *testing.T) {

			files := SplitPath(it.path)

			require.Len(t, files, len(it.items))

			for i, v := range it.items {
				require.Equal(t, v, files[i])
			}

		})
	}

}
