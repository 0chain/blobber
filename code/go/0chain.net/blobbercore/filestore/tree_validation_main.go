//go:build !integration_tests
// +build !integration_tests

package filestore

import "io"

func (v *validationTreeProof) GetMerkleProofOfMultipleIndexes(r io.ReadSeeker, nodesSize int64, startInd, endInd int) (
	[][][]byte, [][]int, error) {
	return v.getMerkleProofOfMultipleIndexes(r, nodesSize, startInd, endInd)
}
