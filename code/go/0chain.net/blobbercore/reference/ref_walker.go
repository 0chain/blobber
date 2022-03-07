package reference

import (
	"path"
	"strings"
)

func NewRefWalkerFromPath(p string) *RefWalker {
	return NewRefWalker(strings.Split(path.Clean(p), "/"))
}

// NewRefWalker wrap dirs as RefWaller
func NewRefWalker(dirs []string) *RefWalker {

	return &RefWalker{
		items:  dirs,
		length: len(dirs),
		index:  0,
	}
}

type RefWalker struct {
	items  []string
	length int
	index  int
}

// Name get current dir name
func (d *RefWalker) Name() string {
	if d == nil {
		return "/"
	}

	// current is root
	if d.index == 0 {
		return "/"
	}

	if d.index < d.length {
		return d.items[d.index]
	}

	return "/"
}

// Path get current dir path
func (d *RefWalker) Path() string {
	if d == nil {
		return ""
	}

	// current is root
	if d.index == 0 {
		return "/"
	}

	if d.index < d.length {
		return "/" + path.Join(d.items[:d.index+1]...)
	}

	return ""
}

// Parent get curerent parent path
func (d *RefWalker) Parent() string {
	if d == nil {
		return ""
	}

	// current is root
	if d.index == 0 {
		return ""
	}

	return "/" + path.Join(d.items[:d.index]...)
}

// Level get current dir level
func (d *RefWalker) Level() int {
	if d == nil {
		return 0
	}

	return d.index + 1
}

// Top move to root dir
func (d *RefWalker) Top() bool {
	if d == nil {
		return false
	}

	d.index = 0

	return true
}

// Last move to last sub dir
func (d *RefWalker) Last() bool {
	if d == nil {
		return false
	}

	if d.length > 0 {
		d.index = d.length - 1
		return true
	}

	return false
}

// Back back to parent dir
func (d *RefWalker) Back() bool {
	if d == nil {
		return false
	}

	i := d.index - 1

	// it is root
	if i < 0 {
		return false
	}

	d.index = i

	return true
}

// Forward go to sub dir
func (d *RefWalker) Forward() bool {
	if d == nil {
		return false
	}

	i := d.index + 1

	// it is root
	if i >= len(d.items) {
		return false
	}

	d.index = i

	return true
}
