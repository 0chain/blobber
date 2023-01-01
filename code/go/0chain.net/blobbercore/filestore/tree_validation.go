// This file will validate challange tree and validation tree and provide merkle path for the provided
// index/es as well.

package filestore

import (
	"crypto/sha256"
	"errors"
	"io"

	"github.com/0chain/gosdk/core/util"
)

const (
	HashSize = 32
	FMTSize  = 65472
	Left     = 0
	Right    = 1
)

type fixedMerkleTree struct {
	*util.FixedMerkleTree
	// merkleRoot []byte
}

func (ft *fixedMerkleTree) CalculateRootAndStoreNodes(f io.Writer) (merkleRoot []byte, err error) {

	nodes := make([][]byte, len(ft.Leaves))
	for i := range ft.Leaves {
		nodes[i] = ft.Leaves[i].GetHashBytes()
	}

	h := sha256.New()
	if len(nodes) == 1 {
		buffer := make([]byte, 64)
		copy(buffer, nodes[0])
		copy(buffer[HashSize:], nodes[0])
		h.Write(nodes[0])
		h.Write(nodes[0])
		merkleRoot = h.Sum(nil)
		return
	}

	for i := 0; i < util.FixedMTDepth; i++ {
		if len(nodes) == 1 {
			break
		}
		buffer := make([]byte, len(nodes)*HashSize)
		var bufInd int
		newNodes := make([][]byte, (len(nodes)+1)/2)
		var nodeInd int
		if len(nodes)&1 == 1 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}
		for j := 0; j < len(nodes); j += 2 {
			h.Reset()
			newBuf := make([]byte, 64)
			copy(newBuf, nodes[j])
			copy(newBuf[HashSize:], nodes[j+1])

			h.Write(newBuf)
			newNodes[nodeInd] = h.Sum(nil)
			nodeInd++
			bufInd += copy(buffer[bufInd:], newBuf)
		}

		_, err = f.Write(buffer)
		if err != nil {
			return nil, err
		}

		nodes = newNodes
	}

	return nodes[0], nil
}

func (f *fixedMerkleTree) GetMerkleProof(idx int, r io.ReaderAt) (proof [][]byte, err error) {
	var levelOffset int
	totalLevelNodes := util.FixedMerkleLeaves
	proof = make([][]byte, util.FixedMTDepth-1)
	for i := 0; i < util.FixedMTDepth-1; i++ {
		b := make([]byte, HashSize)
		var offset int
		if idx&1 == 0 {
			offset = (idx+1)*HashSize + levelOffset
		} else {
			offset = (idx-1)*HashSize + levelOffset
		}

		n, err := r.ReadAt(b, int64(offset))
		if n != HashSize {
			return nil, errors.New("incomplete read")
		}
		if err != nil {
			return nil, err
		}
		proof[i] = b
		levelOffset += totalLevelNodes * HashSize
		totalLevelNodes = totalLevelNodes / 2
	}
	return
}

func getNewFixedMerkleTree() *fixedMerkleTree {
	return &fixedMerkleTree{
		FixedMerkleTree: util.NewFixedMerkleTree(0),
	}
}

type validationTree struct {
	*util.ValidationTree
}

func (v *validationTree) CalculateRootAndStoreNodes(f io.WriteSeeker) (merkleRoot []byte, err error) {
	_, err = f.Seek(FMTSize, io.SeekStart)
	if err != nil {
		return
	}

	nodes := make([][]byte, len(v.GetLeaves()))
	copy(nodes, v.GetLeaves())

	h := sha256.New()
	depth := v.CalculateDepth()

	for i := 0; i < depth; i++ {
		if len(nodes) == 1 {
			break
		}
		buffer := make([]byte, len(nodes)*HashSize)
		var bufInd int
		newNodes := make([][]byte, 0)
		if len(nodes)&1 == 0 {
			for j := 0; j < len(nodes); j += 2 {
				h.Reset()
				h.Write(nodes[j])
				h.Write(nodes[j+1])
				newNodes = append(newNodes, h.Sum(nil))
				bufInd += copy(buffer[bufInd:], nodes[j])
				bufInd += copy(buffer[bufInd:], nodes[j+1])
			}
		} else {
			for j := 0; j < len(nodes); j += 2 {
				h.Reset()
				h.Write(nodes[j])
				h.Write(nodes[j+1])
				newNodes = append(newNodes, h.Sum(nil))
				bufInd += copy(buffer[bufInd:], nodes[j])
				bufInd += copy(buffer[bufInd:], nodes[j+1])
			}
			h.Reset()
			h.Write(nodes[len(nodes)-1])
			newNodes = append(newNodes, h.Sum(nil))
			copy(buffer[bufInd:], nodes[len(nodes)-1])
		}

		_, err := f.Write(buffer)
		if err != nil {
			return nil, err
		}
		nodes = newNodes
	}
	return nodes[0], nil
}

type validationTreeProof struct {
	totalLeaves int
	depth       int
}

func (v *validationTreeProof) GetMerkleProofOfMultipleIndexes(r io.ReaderAt, startInd, endInd int) (
	[][][]byte, [][]int, error) {

	if startInd < 0 || endInd < 0 {
		return nil, nil, errors.New("index cannot be negative")
	}

	if endInd < startInd {
		return nil, nil, errors.New("end index cannot be lesser than start index")
	}

	offsets, leftRightIndexes := v.getFileOffsetsAndNodeIndexes(startInd, endInd)
	offsetInd := 0
	nodeHashes := make([][][]byte, len(leftRightIndexes))
	for i, indexes := range leftRightIndexes {
		for range indexes {
			b := make([]byte, HashSize)
			n, err := r.ReadAt(b, int64(offsets[offsetInd]))
			if err != nil {
				return nil, nil, err
			}
			if n != HashSize {
				return nil, nil, errors.New("invalid hash length")
			}
			nodeHashes[i] = append(nodeHashes[i], b)
			offsetInd++
		}
	}
	return nodeHashes, leftRightIndexes, nil
}

// merge getFileOffsetsAndNodeIndexes and getNodeIndexes
func (v *validationTreeProof) getFileOffsetsAndNodeIndexes(startInd, endInd int) ([]int, [][]int) {

	nodeIndexes, leftRightIndexes := v.getNodeIndexes(startInd, endInd)
	offsets := make([]int, 0)
	totalNodes := 0
	curNodesTot := v.totalLeaves
	for i := 0; i < len(nodeIndexes); i++ {
		for _, ind := range nodeIndexes[i] {
			offsetInd := ind + totalNodes
			offsets = append(offsets, offsetInd*HashSize)
		}
		totalNodes += curNodesTot
		curNodesTot = (curNodesTot + 1) / 2
	}

	return offsets, leftRightIndexes
}

func (v *validationTreeProof) getNodeIndexes(startInd, endInd int) ([][]int, [][]int) {

	indexes := make([][]int, 0)
	leftRightIndexes := make([][]int, 0)
	totalNodes := v.totalLeaves
	for i := v.depth - 1; i >= 0; i-- {
		if startInd == 0 && endInd == totalNodes-1 {
			break
		}

		nodeOffsets := make([]int, 0)
		lftRtInd := make([]int, 0)
		if startInd%2 != 0 {
			nodeOffsets = append(nodeOffsets, startInd-1)
			lftRtInd = append(lftRtInd, Left)
		}

		if endInd != totalNodes-1 && endInd%2 == 0 {
			nodeOffsets = append(nodeOffsets, endInd+1)
			lftRtInd = append(lftRtInd, Right)
		}

		indexes = append(indexes, nodeOffsets)
		leftRightIndexes = append(leftRightIndexes, lftRtInd)
		startInd = startInd / 2
		endInd = endInd / 2
		totalNodes = (totalNodes + 1) / 2
	}
	return indexes, leftRightIndexes
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

func (c *commitHasher) Finalize() error {
	err := c.fmt.Finalize()
	if err != nil {
		return err
	}

	return c.vt.Finalize()
}
