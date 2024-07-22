// This file will validate challange tree and validation tree and provide merkle path for the provided
// index/es as well.

package filestore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/seqpriorityqueue"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/util"
	"github.com/minio/sha256-simd"
	"go.uber.org/zap"
)

const (
	KB = 1024
	MB = KB * KB
)

const (
	HashSize = 32
	// FMTSize is the size of tree nodes of fixed merkle tree excluding root node
	FMTSize     = 65472
	FMTDisKRead = 10 * MB
)

// fixedMerkleTree is used to verify fixed merkle tree root and store tree nodes to the file
type fixedMerkleTree struct {
	*util.FixedMerkleTree
}

// getNodesSize will calculate the size required to store tree nodes excluding root node.
func getNodesSize(dataSize, merkleLeafSize int64) int64 {
	totalLeaves := (dataSize + merkleLeafSize - 1) / merkleLeafSize
	if totalLeaves == 1 {
		return 0
	}
	totalNodes := totalLeaves
	for totalLeaves > 2 {
		totalLeaves = (totalLeaves + 1) / 2
		totalNodes += totalLeaves

	}
	return totalNodes * HashSize
}

func calculateLeaves(dataSize int64) int {
	return int(math.Ceil(float64(dataSize) / util.MaxMerkleLeavesSize))
}

func calculateDepth(totalLeaves int) int {
	return int(math.Ceil(math.Log2(float64(totalLeaves)))) + 1
}

// CalculateRootAndStoreNodes will calculate all the intermediate nodes and write it to
// f in bytes format
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

			_, _ = h.Write(buffer[prevBufLen:bufLen])
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

// fixedMerkleTreeProof is used to calculate merkle proof of certain index and also get the
// content in batches.
type fixedMerkleTreeProof struct {
	idx      int
	dataSize int64
	offset   int64
}

func NewFMTPRoof(idx int, dataSize int64) *fixedMerkleTreeProof {
	return &fixedMerkleTreeProof{
		idx:      idx,
		dataSize: dataSize,
	}
}

// CalculateLeafContentLevelForIndex is used to calculate total levels or rows of a column of respective
// index. So if datasize is 64KB + 1 bytes then 0th index has level 2 and all other index will have level 1.
func (fp *fixedMerkleTreeProof) CalculateLeafContentLevelForIndex() int {
	levelFor0Idx := (fp.dataSize + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize
	if fp.dataSize%util.MaxMerkleLeavesSize == 0 || fp.idx == 0 {
		return int(levelFor0Idx)
	}

	prevRowSize := (levelFor0Idx - 1) * util.MaxMerkleLeavesSize
	curRowSize := fp.dataSize - prevRowSize

	n := int(curRowSize+util.MerkleChunkSize-1) / util.MerkleChunkSize

	if fp.idx >= n {
		return int(levelFor0Idx) - 1
	}
	return int(levelFor0Idx)
}

// GetMerkleProof is used to get merkle proof of leaf or index to be specific.
func (fp fixedMerkleTreeProof) GetMerkleProof(r io.ReaderAt) (proof [][]byte, err error) {
	var levelOffset int
	totalLevelNodes := util.FixedMerkleLeaves
	proof = make([][]byte, util.FixedMTDepth-1)
	b := make([]byte, FMTSize)
	n, err := r.ReadAt(b, fp.offset)
	if n != FMTSize {
		return nil, fmt.Errorf("invalid fixed merkle tree size: %d", n)
	}
	if err != nil {
		return nil, err
	}

	var offset int
	idx := fp.idx

	for i := 0; i < util.FixedMTDepth-1; i++ {
		if idx&1 == 0 {
			offset = (idx+1)*HashSize + levelOffset // take right neighbour for left(even) index
		} else {
			offset = (idx-1)*HashSize + levelOffset // take left neighbour for right(odd)  index
		}

		proof[i] = b[offset : offset+HashSize]
		levelOffset += totalLevelNodes * HashSize
		totalLevelNodes = totalLevelNodes / 2
		idx = idx / 2
	}
	return
}

// GetLeafContent is used to retrieve leaf content of respective index. The data is read in
// batch of 10 MB. It may be increased to 100MB if disk read make challenge verification slow.
// r should have offset seeked already
func (fp *fixedMerkleTreeProof) GetLeafContent(r io.Reader) (proofByte []byte, err error) {
	levels := fp.CalculateLeafContentLevelForIndex() + 1
	proofByte = make([]byte, levels*util.MerkleChunkSize)
	var proofWritten int
	idxOffset := fp.idx * util.MerkleChunkSize
	idxLimit := idxOffset + util.MerkleChunkSize
	b := make([]byte, FMTDisKRead)
	for {
		n, err := r.Read(b)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			if n == 0 {
				break
			}
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
			}

			proofWritten += copy(proofByte[proofWritten:proofWritten+util.MerkleChunkSize],
				data[idxOffset:idxLimit])
		}
	}
	return proofByte[:proofWritten], nil
}

func getNewFixedMerkleTree() *fixedMerkleTree {
	return &fixedMerkleTree{
		FixedMerkleTree: util.NewFixedMerkleTree(),
	}
}

// validationTree is used to calculate root and store validation tree nodes excluding root node.
type validationTree struct {
	*util.ValidationTree
}

// CalculateRootAndStoreNodes is used to calculate root and write intermediate nodes excluding root
// node to f
func (v *validationTree) CalculateRootAndStoreNodes(f io.Writer, dataSize int64) (merkleRoot []byte, err error) {

	nodes := make([][]byte, len(v.GetLeaves()))
	copy(nodes, v.GetLeaves())

	h := sha256.New()
	depth := v.CalculateDepth()

	s := getNodesSize(dataSize, util.MaxMerkleLeavesSize)
	buffer := make([]byte, s)
	var bufInd int
	for i := 0; i < depth; i++ {
		if len(nodes) == 1 {
			break
		}
		newNodes := make([][]byte, 0)
		if len(nodes)&1 == 0 {
			for j := 0; j < len(nodes); j += 2 {
				h.Reset()
				prevBufInd := bufInd
				bufInd += copy(buffer[bufInd:], nodes[j])
				bufInd += copy(buffer[bufInd:], nodes[j+1])

				_, _ = h.Write(buffer[prevBufInd:bufInd])
				newNodes = append(newNodes, h.Sum(nil))
			}
		} else {
			for j := 0; j < len(nodes)-1; j += 2 {
				h.Reset()
				prevBufInd := bufInd
				bufInd += copy(buffer[bufInd:], nodes[j])
				bufInd += copy(buffer[bufInd:], nodes[j+1])

				_, _ = h.Write(buffer[prevBufInd:bufInd])
				newNodes = append(newNodes, h.Sum(nil))
			}
			h.Reset()
			prevBufInd := bufInd
			bufInd += copy(buffer[bufInd:], nodes[len(nodes)-1])
			_, _ = h.Write(buffer[prevBufInd:bufInd])
			newNodes = append(newNodes, h.Sum(nil))
		}

		nodes = newNodes
	}

	_, err = f.Write(buffer)
	if err != nil {
		return nil, err
	}

	return nodes[0], nil
}

// validationTreeProof is used to calculate and retrieve merkle path
type validationTreeProof struct {
	totalLeaves int
	depth       int
	dataSize    int64
	offset      int64
}

// GetMerkleProofOfMultipleIndexes will get minimum proof based on startInd and endInd values.
// If endInd - startInd is whole file then no proof is required at all.
// startInd and endInd is taken as closed interval. So to get proof for data at index 0 both startInd
// and endInd would be 0.
func (v *validationTreeProof) getMerkleProofOfMultipleIndexes(r io.ReadSeeker, nodesSize int64, startInd, endInd int) (
	[][][]byte, [][]int, error) {

	if startInd < 0 || endInd < 0 {
		return nil, nil, errors.New("index cannot be negative")
	}

	v.totalLeaves = calculateLeaves(v.dataSize)
	if endInd > v.totalLeaves {
		endInd = v.totalLeaves - 1
	}

	if endInd < startInd {
		return nil, nil, errors.New("end index cannot be lesser than start index")
	}

	if v.depth == 0 {
		v.depth = calculateDepth(v.totalLeaves)
	}

	// nodesSize := getNodesSize(v.dataSize, util.MaxMerkleLeavesSize)
	offsets, leftRightIndexes := v.getFileOffsetsAndNodeIndexes(startInd, endInd)
	nodesData := make([]byte, nodesSize)
	_, err := r.Seek(v.offset, io.SeekStart)
	if err != nil {
		return nil, nil, err
	}

	_, err = r.Read(nodesData)
	if err != nil {
		return nil, nil, err
	}

	offsetInd := 0
	nodeHashes := make([][][]byte, len(leftRightIndexes))
	for i, indexes := range leftRightIndexes {
		for range indexes {
			b := make([]byte, HashSize)
			off := offsets[offsetInd]
			n := copy(b, nodesData[off:off+HashSize])
			if n != HashSize {
				return nil, nil, errors.New("invalid hash length")
			}
			nodeHashes[i] = append(nodeHashes[i], b)
			offsetInd++
		}
	}
	return nodeHashes, leftRightIndexes, nil
}

// getFileOffsetsAndNodeIndexes
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

// getNodeIndexes returns two slices.
//  1. NodeOffsets will return offset index of node in each level. Each level starts with index zero.
//  2. leftRightIndexes will return whether the node should be appended to the left or right
//     with other hash
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
		if startInd&1 == 1 {
			nodeOffsets = append(nodeOffsets, startInd-1)
			lftRtInd = append(lftRtInd, util.Left)
		}

		if endInd != totalNodes-1 && endInd&1 == 0 {
			nodeOffsets = append(nodeOffsets, endInd+1)
			lftRtInd = append(lftRtInd, util.Right)
		}

		indexes = append(indexes, nodeOffsets)
		leftRightIndexes = append(leftRightIndexes, lftRtInd)
		startInd = startInd / 2
		endInd = endInd / 2
		totalNodes = (totalNodes + 1) / 2
	}
	return indexes, leftRightIndexes
}

func getNewValidationTree(dataSize int64) *validationTree {
	return &validationTree{
		ValidationTree: util.NewValidationTree(dataSize),
	}
}

// commitHasher is used to calculate and store tree nodes for fixed merkle tree and
// validation tree when client commits file with the writemarker.
type CommitHasher struct {
	fmt           *fixedMerkleTree
	vt            *validationTree
	md5hasher     hash.Hash
	isInitialized bool
	doneChan      chan struct{}
	hashErr       error
	dataSize      int64
}

func NewCommitHasher(dataSize int64) *CommitHasher {
	c := new(CommitHasher)
	c.md5hasher = md5.New()
	c.isInitialized = true
	c.doneChan = make(chan struct{})
	c.dataSize = dataSize
	return c
}

func (c *CommitHasher) Start(ctx context.Context, connID, allocID, fileName, filePathHash string, seqPQ *seqpriorityqueue.SeqPriorityQueue) {
	defer close(c.doneChan)
	tempFilePath := GetFileStore().GetTempFilePath(allocID, connID, fileName, filePathHash)
	f, err := os.Open(tempFilePath)
	if err != nil {
		logging.Logger.Error("hasher_open", zap.Error(err), zap.String("tempFilePath", tempFilePath))
		c.hashErr = err
		return
	}
	defer f.Close()
	var toFinalize bool
	var totalWritten int64

	for {
		select {
		case <-ctx.Done():
			c.hashErr = ctx.Err()
			return
		default:
		}
		pq := seqPQ.Popup()
		if pq.Offset+pq.DataBytes == c.dataSize || pq.IsFinal {
			// If dataBytes and offset is equal to data size then it is the last data to be read from the file or context is cancelled
			// Check if ctx is done
			select {
			case <-ctx.Done():
				c.hashErr = ctx.Err()
				return
			default:
			}
			toFinalize = true
		} else if pq.DataBytes == 0 {
			continue
		}
		logging.Logger.Info("hasher_pop", zap.Int64("offset", pq.Offset), zap.Int64("dataBytes", pq.DataBytes), zap.Any("toFinalize", toFinalize), zap.Int64("dataSize", c.dataSize), zap.String("filename", fileName), zap.Int64("totalWritten", totalWritten))
		bufSize := 2 * BufferSize
		if pq.DataBytes < int64(bufSize) {
			bufSize = int(pq.DataBytes)
		}
		buf := make([]byte, bufSize)
		for pq.DataBytes > 0 {
			if pq.DataBytes < int64(bufSize) {
				buf = buf[:pq.DataBytes]
			}
			n, err := f.ReadAt(buf, pq.Offset)
			if err != nil && !errors.Is(err, io.EOF) {
				c.hashErr = err
				return
			}
			pq.DataBytes -= int64(n)
			pq.Offset += int64(n)
			totalWritten += int64(n)
			_, err = c.md5hasher.Write(buf[:n])
			if err != nil {
				logging.Logger.Error("hasher_write", zap.Error(err), zap.Int("n", n), zap.Int64("offset", pq.Offset), zap.Int64("dataBytes", pq.DataBytes), zap.Int64("dataSize", c.dataSize), zap.String("filename", fileName), zap.Int64("totalWritten", totalWritten))
				c.hashErr = err
				return
			}
		}
		buf = nil
		if toFinalize {
			return
		}
	}
}

func (c *CommitHasher) Wait(ctx context.Context, connID, allocID, fileName, filePathHash string) error {
	select {
	case <-c.doneChan:
		return c.hashErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *CommitHasher) Write(b []byte) (int, error) {
	if !c.isInitialized || c.fmt == nil || c.vt == nil {
		return 0, errors.New("commit hasher is not initialized")
	}

	var (
		wg      sync.WaitGroup
		errChan = make(chan error, 2)
		n       int
	)
	wg.Add(2)
	go func() {
		_, err := c.fmt.Write(b)
		if err != nil {
			errChan <- err
		}

		wg.Done()
	}()
	go func() {
		written, err := c.vt.Write(b)
		if err != nil {
			errChan <- err
		}
		n = written
		wg.Done()
	}()
	wg.Wait()
	close(errChan)
	for err := range errChan {
		return n, err
	}
	return n, nil
}

func (c *CommitHasher) Finalize() error {
	var (
		wg      sync.WaitGroup
		errChan = make(chan error, 2)
	)
	wg.Add(2)
	go func() {
		err := c.fmt.Finalize()
		if err != nil {
			errChan <- err
		}
		wg.Done()
	}()
	go func() {
		err := c.vt.Finalize()
		if err != nil {
			errChan <- err
		}
		wg.Done()
	}()
	wg.Wait()
	close(errChan)
	for err := range errChan {
		return err
	}
	return nil
}

func (c *CommitHasher) GetFixedMerkleRoot() string {
	return c.fmt.GetMerkleRoot()
}

func (c *CommitHasher) GetValidationMerkleRoot() string {
	return hex.EncodeToString(c.vt.GetValidationRoot())
}

func (c *CommitHasher) GetMd5Hash() string {
	return hex.EncodeToString(c.md5hasher.Sum(nil))
}
