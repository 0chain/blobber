// This file will validate challange tree and validation tree and provide merkle path for the provided
// index/es as well.

package filestore

import (
	"io"

	"github.com/0chain/gosdk/core/util"
)

const (
	FMTSize = 65472
)

type fixedMerkleTree struct {
	*util.FixedMerkleTree
}

func (f *fixedMerkleTree) CalculateRootAndStoreNodes() {

}

func (f *fixedMerkleTree) GetMerkleProof(idx int) ([][]byte, error) {
	return nil, nil
}

func (f *fixedMerkleTree) calculateOffsets() {

}

func getNewFixedMerkleTree() *fixedMerkleTree {
	return &fixedMerkleTree{
		FixedMerkleTree: util.NewFixedMerkleTree(0),
	}
}

type validationTree struct {
	*util.ValidationTree
}

func (v *validationTree) CalculateRootAndStoreNodes() {

}

func (v *validationTree) GetMerkleProofOfMultipleIndexes(r io.ReaderAt, startInd, endInd int) (
	[][][]byte, [][]int, error) {

	return nil, nil, nil
}

func getNewValidationTree() *validationTree {
	return &validationTree{
		ValidationTree: util.NewValidationTree(0),
	}
}

type commitHasher struct {
	fmt           *fixedMerkleTree
	vt            *validationTree
	writer        io.Writer
	isInitialized bool
}

func GetNewCommitHasher(dataSize int64) *commitHasher {
	c := new(commitHasher)
	c.fmt = getNewFixedMerkleTree()
	c.vt = getNewValidationTree()
	c.writer = io.MultiWriter(c.fmt, c.vt)
	c.isInitialized = true
	return nil
}

func (c *commitHasher) Write(b []byte) (int, error) {
	return c.writer.Write(b)
}
