package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// The AttributesChange represents file attributes change.
type AttributesChange struct {
	ConnectionID string                `json:"connection_id"`
	AllocationID string                `json:"allocation_id"`
	Path         string                `json:"path"`
	Attributes   *reference.Attributes `json:"attributes"` // new attributes
}

// ApplyChange processes the attributes changes.
func (ac *AttributesChange) ApplyChange(ctx context.Context, _ *AllocationChange, allocRoot string) (ref *reference.Ref, err error) {
	var path, _ = filepath.Split(ac.Path)
	path = filepath.Clean(path)

	// root reference
	ref, err = reference.GetReferencePath(ctx, ac.AllocationID, ac.Path)
	if err != nil {
		return nil, common.NewErrorf("process_attrs_update",
			"getting root reference path: %v", err)
	}

	var (
		tSubDirs  = reference.GetSubDirsFromPath(path)
		dirRef    = ref
		treelevel = 0
	)

	for treelevel < len(tSubDirs) {
		var found bool
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY && treelevel < len(tSubDirs) {
				if child.Name == tSubDirs[treelevel] {
					dirRef, found = child, true
					break
				}
			}
		}
		if found {
			treelevel++
		} else {
			return nil, common.NewError("process_attrs_update",
				"invalid reference path from the blobber")
		}
	}

	var idx = -1
	for i, child := range dirRef.Children {
		if child.Type == reference.FILE && child.Path == ac.Path {
			idx = i
			break
		}
	}

	if idx < 0 {
		Logger.Error("error in file attributes update", zap.Any("change", ac))
		return nil, common.NewError("process_attrs_update",
			"file to update not found in blobber")
	}

	var existingRef = dirRef.Children[idx]
	existingRef.WriteMarker = allocRoot
	if err = existingRef.SetAttributes(ac.Attributes); err != nil {
		return nil, common.NewErrorf("process_attrs_update",
			"setting new attributes: %v", err)
	}

	if _, err = ref.CalculateHash(ctx, true); err != nil {
		return nil, common.NewErrorf("process_attrs_update",
			"saving updated reference: %v", err)
	}

	stats.FileUpdated(ctx, existingRef.ID)
	return
}

// Marshal to JSON-string.
func (ac *AttributesChange) Marshal() (val string, err error) {
	var b []byte
	if b, err = json.Marshal(ac); err != nil {
		return
	}
	return string(b), nil
}

// Unmarshal from given JSON-string.
func (ac *AttributesChange) Unmarshal(val string) (err error) {
	err = json.Unmarshal([]byte(val), ac)
	return
}

// The DeleteTempFile returns OperationNotApplicable error.
func (ac *AttributesChange) DeleteTempFile() (err error) {
	return OperationNotApplicable
}

// The CommitToFileStore does nothing.
func (ac *AttributesChange) CommitToFileStore(_ context.Context) (err error) {
	return
}
