package reference

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefWalker(t *testing.T) {
	list := []struct {
		TestName   string
		TestPath   string
		WalkFunc   func(w *RefWalker) bool
		WalkResult bool
		Name       string
		Level      int
		Path       string
		Parent     string
	}{
		{TestName: "invalid_path", TestPath: "", WalkFunc: func(w *RefWalker) bool {
			return w.Top()
		}, WalkResult: true, Name: "/", Level: 1, Path: "/", Parent: ""},

		{TestName: "top", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			return w.Top()
		}, WalkResult: true, Name: "/", Level: 1, Path: "/", Parent: ""},

		{TestName: "forward_2", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			w.Top()
			return w.Forward()
		}, WalkResult: true, Name: "d1", Level: 2, Path: "/d1", Parent: "/"},

		{TestName: "forward_3", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			w.Top()
			w.Forward()
			return w.Forward()
		}, WalkResult: true, Name: "d2", Level: 3, Path: "/d1/d2", Parent: "/d1"},

		{TestName: "forward_last", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			w.Last()
			return w.Forward()
		}, WalkResult: false, Name: "d3", Level: 4, Path: "/d1/d2/d3", Parent: "/d1/d2"},

		{TestName: "last", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			return w.Last()
		}, WalkResult: true, Name: "d3", Level: 4, Path: "/d1/d2/d3", Parent: "/d1/d2"},

		{TestName: "back_2", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			w.Last()
			return w.Back()
		}, WalkResult: true, Name: "d2", Level: 3, Path: "/d1/d2", Parent: "/d1"},

		{TestName: "back_3", TestPath: "/d1/d2/d3", WalkFunc: func(w *RefWalker) bool {
			w.Last()
			w.Back()
			return w.Back()
		}, WalkResult: true, Name: "d1", Level: 2, Path: "/d1", Parent: "/"},

		{TestName: "back_3_with_invalid_dir", TestPath: `/d1///d2/d3`, WalkFunc: func(w *RefWalker) bool {
			w.Last()
			w.Back()
			return w.Back()
		}, WalkResult: true, Name: "d1", Level: 2, Path: "/d1", Parent: "/"},
	}

	for _, it := range list {
		t.Run(it.TestName, func(test *testing.T) {

			dirs := strings.Split(path.Clean(it.TestPath), "/")

			rw := NewRefWalker(dirs)
			result := it.WalkFunc(rw)

			r := require.New(test)

			r.Equal(it.WalkResult, result)
			name, ok := rw.Name()
			r.True(ok)
			r.Equal(it.Name, name)

			r.Equal(it.Level, rw.Level())

			path, ok := rw.Path()
			r.True(ok)
			r.Equal(it.Path, path)

			parentPath, ok := rw.Parent()
			r.True(ok)
			r.Equal(it.Parent, parentPath)

		})
	}

}
