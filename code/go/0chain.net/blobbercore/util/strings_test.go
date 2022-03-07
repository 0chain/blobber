package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitFiles(t *testing.T) {

	tests := []struct {
		name  string
		path  string
		files []string
	}{
		{
			name:  "empty root",
			path:  "/",
			files: []string{},
		},
		{
			name:  "only 1 directory level deep",
			path:  "/file1",
			files: []string{"file1"},
		},
		{
			name:  "nested directories",
			path:  "/dir1/dir2/dir3/file1",
			files: []string{"dir1", "dir2", "dir3", "file1"},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(t *testing.T) {

			files := SplitFiles(it.path)

			require.Len(t, files, len(it.files))

			for i, v := range it.files {
				require.Equal(t, v, files[i])
			}

		})
	}

}
