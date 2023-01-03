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
	KB = 1024
	MB = KB * KB
)

const (
	FMTLeafContentSize = 64 * KB
	HashSize           = 32
	FMTSize            = 65472
	Left               = 0
	Right              = 1
	// ValidationTreeReservedBytes will store three fields required for
	ValidationTreeReservedBytes = 8 * 3
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

	buffer := make([]byte, FMTSize)
	var bufLen int
	h := sha256.New()

	for i := 0; i < util.FixedMTDepth; i++ {
		if len(nodes) == 1 {
			break
		}
		buffer := make([]byte, len(nodes)*HashSize)
		newNodes := make([][]byte, (len(nodes)+1)/2)
		var nodeInd int
		if len(nodes)&1 == 1 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}
		for j := 0; j < len(nodes); j += 2 {
			h.Reset()
			prevBufLen := bufLen
			bufLen += copy(buffer[bufLen:bufLen+HashSize], nodes[j])
			bufLen += copy(buffer[bufLen:bufLen+HashSize], nodes[j+1])

			h.Write(buffer[prevBufLen:bufLen])
			newNodes[nodeInd] = h.Sum(nil)
			nodeInd++
		}

		nodes = newNodes
	}

	_, err = f.Write(buffer)
	if err != nil {
		return nil, err
	}

	return nodes[0], nil
}

type fixedMerkleTreeProof struct {
	idx      int
	dataSize int64
}

func NewFMTPRoof(idx int, dataSize int64) *fixedMerkleTreeProof {
	return &fixedMerkleTreeProof{
		idx:      idx,
		dataSize: dataSize,
	}
}

func (fp *fixedMerkleTreeProof) CalculateLeafContentLevelForIndex() int {
	levelFor0Idx := (fp.dataSize + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize
	if fp.dataSize%util.MaxMerkleLeavesSize == 0 || fp.idx == 0 {
		return int(levelFor0Idx)
	}

	prevRowSize := (levelFor0Idx - 1) * util.MaxMerkleLeavesSize
	curRowSize := fp.dataSize - prevRowSize

	n := int(curRowSize+util.MerkleChunkSize-1) / util.MerkleChunkSize

	if fp.idx > n {
		return int(levelFor0Idx) - 1
	}
	return int(levelFor0Idx)
}

func (fixedMerkleTreeProof) GetMerkleProof(idx int, r io.ReaderAt) (proof [][]byte, err error) {
	var levelOffset int
	totalLevelNodes := util.FixedMerkleLeaves
	proof = make([][]byte, util.FixedMTDepth-1)
	b := make([]byte, FMTSize)
	n, err := r.ReadAt(b, io.SeekStart)
	if n != FMTSize {
		return nil, errors.New("incomplete read")
	}
	if err != nil {
		return nil, err
	}

	var offset int

	for i := 0; i < util.FixedMTDepth-1; i++ {
		if idx&1 == 0 {
			offset = (idx+1)*HashSize + levelOffset
		} else {
			offset = (idx-1)*HashSize + levelOffset
		}

		proof[i] = b[offset : offset+HashSize]
		levelOffset += totalLevelNodes * HashSize
		totalLevelNodes = totalLevelNodes / 2
	}
	return
}

// r should have offset seeked already
func (fp *fixedMerkleTreeProof) GetLeafContent(idx int, r io.Reader) (proofByte []byte, err error) {
	levels := fp.CalculateLeafContentLevelForIndex() + 1
	proofByte = make([]byte, levels*util.MerkleChunkSize)
	var proofWritten int
	idxOffset := idx * util.MerkleChunkSize
	idxLimit := idxOffset + util.MerkleChunkSize
	b := make([]byte, 10*MB)
	var shouldBreak bool
	for !shouldBreak {
		n, err := r.Read(b)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			shouldBreak = true
		}
		b = b[:n]

		for i := 0; i < len(b); i += util.MaxMerkleLeavesSize {
			endIndex := i + util.MaxMerkleLeavesSize
			if endIndex > len(b) {
				endIndex = len(b)
			}
			data := b[i:endIndex]
			if idxLimit > len(data) {
				idxLimit = len(data)
				if idxOffset > len(data) {
					idxOffset = len(data)
				}
				shouldBreak = true
			}

			proofWritten += copy(proofByte[proofWritten:proofWritten+util.MerkleChunkSize],
				data[idxOffset:idxLimit])
		}
	}
	return proofByte[:proofWritten], nil
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
