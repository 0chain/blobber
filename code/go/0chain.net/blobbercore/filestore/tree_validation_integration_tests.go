//go:build integration_tests
// +build integration_tests

package filestore

import (
	"io"

	crpc "github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func (v *validationTreeProof) GetMerkleProofOfMultipleIndexes(r io.ReadSeeker, nodesSize int64, startInd, endInd int) (
	[][][]byte, [][]int, error) {
	nodes, indexes, err := v.getMerkleProofOfMultipleIndexes(r, nodesSize, startInd, endInd)

	state := crpc.Client().State()
	if state == nil {
		return nodes, indexes, err
	}

	if state.MissUpDownload {
		logging.Logger.Debug("miss up/download",
			zap.Bool("miss_up_download", state.MissUpDownload),
			zap.Int("start_ind", startInd),
			zap.Int("end_ind", endInd),
			zap.Int64("nodes_size", nodesSize),
			zap.Int("output_nodes_size", len(nodes)),
			zap.Int("output_indexes_size", len(indexes)),
			zap.Any("nodes", nodes),
			zap.Any("indexes", indexes),
		)

		for i := range nodes {
			nodes[i][0] = []byte("wrong data")
		}

		for i := range indexes {
			indexes[i][0] = 0
		}
	}

	return nodes, indexes, err
}