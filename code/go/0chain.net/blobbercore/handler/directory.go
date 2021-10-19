package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"
)

// CreateDir is the handler to respond to create dir for allocation
func CreateDir(ctx *Context) (interface{}, error) {

	db := ctx.Store.GetDB()

	name := ctx.Request.FormValue("name")

	if len(name) == 0 {
		return false, errors.Throw(constants.ErrBadRequest, "name")
	}

	rw := reference.NewRefWalkerFromPath(name)
	rw.Last()

	err := db.Transaction(func(tx *gorm.DB) error {
		for {

			ok, err := reference.Exists(ctx, tx, ctx.AllocationTx, rw.Path())

			if ok {
				return nil
			}

			// raw db error
			if err != nil && !errors.Is(err, constants.ErrEntityNotFound) {
				return errors.ThrowLog(err.Error(), constants.ErrBadDatabaseOperation, name)
			}

			// doesn't exists, create it, and check its parent

			dir := reference.NewDirectoryRef()

			dir.ActualFileSize = 0
			dir.AllocationID = ctx.AllocationTx
			dir.MerkleRoot = ""

			dir.PathLevel = rw.Level()
			dir.Path = rw.Path()
			dir.ParentPath = rw.Parent()
			dir.Name = rw.Name()
			dir.Size = 0
			dir.NumBlocks = 0

			dir.WriteMarker = ""

			dir.LookupHash = reference.GetReferenceLookup(ctx.AllocationTx, dir.Path)
			dir.PathHash = dir.LookupHash

			_, err = dir.CalculateHash(ctx, false)
			if err != nil {
				return errors.ThrowLog(err.Error(), constants.ErrInternal, name)
			}

			err = tx.Create(dir).Error

			if err != nil {
				return errors.ThrowLog(err.Error(), constants.ErrBadDatabaseOperation, name)
			}

			if !rw.Back() {
				return nil
			}

		}
	})

	if err != nil {
		return false, err
	}

	return true, nil
}
