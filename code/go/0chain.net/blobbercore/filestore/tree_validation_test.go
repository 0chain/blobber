package filestore

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/core/util"
	"github.com/stretchr/testify/require"
)

type storeLeaf struct {
	hash []byte
}

func (s *storeLeaf) GetHash() string {
	return hex.EncodeToString(s.hash)
}

func (s *storeLeaf) GetHashBytes() []byte {
	return s.hash
}

func (s *storeLeaf) Write([]byte) (int, error) {
	return 0, nil
}

func TestFixedMerkleTreeWrite(t *testing.T) {
	for i := 0; i < 100; i++ {
		var n int64
		for {
			n = rand.Int63n(50 * MB)
			if n != 0 {
				break
			}
		}

		b := make([]byte, n)
		n1, err := rand.Read(b)
		require.NoError(t, err)
		require.EqualValues(t, n, n1)

		t.Log("testing with data-size: ", n)

		ft := getNewFixedMerkleTree()
		n1, err = ft.Write(b)
		require.NoError(t, err)
		require.EqualValues(t, n, n1)
		err = ft.Finalize()
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		merkleRoot, err := ft.CalculateRootAndStoreNodes(buf)
		require.NoError(t, err)

		leaves := make([]util.Hashable, util.FixedMerkleLeaves)
		for i := 0; i < util.FixedMerkleLeaves; i++ {
			p := make([]byte, HashSize)
			n, err := buf.Read(p)
			require.NoError(t, err)
			require.EqualValues(t, HashSize, n)

			leaves[i] = &storeLeaf{
				hash: p,
			}
		}

		p := make([]byte, HashSize)
		n2, err := buf.Read(p)
		require.NoError(t, err)
		require.EqualValues(t, n2, HashSize)

		mt := util.FixedMerkleTree{
			Leaves: leaves,
		}

		computedRoot := mt.GetMerkleRoot()
		require.Equal(t, computedRoot, hex.EncodeToString(merkleRoot))
	}
}

func TestFixedMerkleTreeProof(t *testing.T) {
	var size int64
	var index int
	for i := 0; i < 100; i++ {
		for {
			size = rand.Int63n(50 * MB)
			if size != 0 {
				break
			}

			index = rand.Intn(1024)
		}

		t.Run(fmt.Sprintf("Merkleproof with size=%d", size), func(t *testing.T) {
			filename := "merkleproof"
			defer func() {
				os.Remove(filename)
			}()

			f, err := os.Create(filename)
			require.NoError(t, err)

			b := make([]byte, size)
			n, err := rand.Read(b)
			require.EqualValues(t, size, n)
			require.NoError(t, err)

			ft := getNewFixedMerkleTree()
			n, err = ft.Write(b)

			require.EqualValues(t, n, size)
			require.NoError(t, err)

			err = ft.Finalize()
			require.NoError(t, err)

			merkleRoot, err := ft.CalculateRootAndStoreNodes(f)
			require.NoError(t, err)

			n, err = f.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)

			finfo, err := r.Stat()
			require.NoError(t, err)
			require.EqualValues(t, size+FMTSize, finfo.Size())

			fm := fixedMerkleTreeProof{
				idx:      index,
				dataSize: int64(size),
			}

			proof, err := fm.GetMerkleProof(r)
			require.NoError(t, err)

			n1, err := r.Seek(FMTSize, io.SeekStart)
			require.NoError(t, err)
			require.EqualValues(t, FMTSize, n1)
			proofByte, err := fm.GetLeafContent(r)
			require.NoError(t, err)

			leafHash := encryption.RawHash(proofByte)
			ftp := util.FixedMerklePath{
				LeafHash: leafHash,
				RootHash: merkleRoot,
				Nodes:    proof,
				LeafInd:  fm.idx,
			}

			require.True(t, ftp.VerifyMerklePath())
		})

	}
}

func TestValidationTreeWrite(t *testing.T) {
	for i := 0; i < 100; i++ {
		size := getRandomSize(50 * MB)
		t.Run(fmt.Sprintf("testing with data-size: %d", size), func(t *testing.T) {
			b := make([]byte, size)
			n, err := rand.Read(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			vt := getNewValidationTree(int64(size))
			n, err = vt.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			err = vt.Finalize()
			require.NoError(t, err)

			merkleRoot := vt.GetValidationRoot()
			require.NotNil(t, merkleRoot)

			filename := "merkleproof"
			defer func() {
				os.Remove(filename)
			}()

			f, err := os.Create(filename)
			require.NoError(t, err)

			merkleRoot1, err := vt.CalculateRootAndStoreNodes(f)
			require.NoError(t, err)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)
			n1, err := r.Seek(FMTSize, io.SeekStart)
			require.NoError(t, err)
			require.EqualValues(t, FMTSize, n1)

			totalLeaves := int((size + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize)
			nodes := make([]byte, totalLeaves*HashSize)
			n, err = r.Read(nodes)
			require.NoError(t, err)
			require.EqualValues(t, len(nodes), n)

			leaves := make([][]byte, totalLeaves)
			for i := 0; i < totalLeaves; i++ {
				off := i * HashSize
				leaves[i] = nodes[off : off+HashSize]
			}

			v := util.ValidationTree{}
			v.SetLeaves(leaves)
			merkleRoot2 := v.GetValidationRoot()
			require.True(t, bytes.Equal(merkleRoot, merkleRoot2))
			require.True(t, bytes.Equal(merkleRoot, merkleRoot1))
		})

	}
}

func TestValidationMerkleProof(t *testing.T) {
	for i := 0; i < 100; i++ {
		var size int64
		for {
			size = rand.Int63n(50 * MB)
			if size != 0 {
				break
			}
		}

		startInd, endInd := getRandomIndexRange(size)

		t.Run(fmt.Sprintf("Merkle proof test with size: %d, startInd: %d and endInd: %d",
			size, startInd, endInd), func(t *testing.T) {

			b := make([]byte, size)
			n, err := rand.Read(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			vt := getNewValidationTree(int64(size))
			n, err = vt.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			err = vt.Finalize()
			require.NoError(t, err)

			filename := "validationmerkleproof"
			defer func() {
				os.Remove(filename)
			}()

			f, err := os.Create(filename)
			require.NoError(t, err)

			merkleRoot, err := vt.CalculateRootAndStoreNodes(f)
			require.NoError(t, err)

			n, err = f.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)

			finfo, err := r.Stat()
			require.NoError(t, err)

			totalNodes := getValidationTreeTotalNodes(int(size))
			nodesSize := totalNodes * HashSize

			expectedSize := FMTSize + int(size) + nodesSize
			require.EqualValues(t, expectedSize, finfo.Size(), fmt.Sprint("Diff is: ", finfo.Size()-int64(expectedSize)))

			vp := validationTreeProof{
				totalLeaves: int((size + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize),
				dataSize:    int64(size),
			}

			t.Logf("StartInd: %d; endInd: %d", startInd, endInd)
			nodeHashes, indexes, err := vp.GetMerkleProofOfMultipleIndexes(r, startInd, endInd)
			require.NoError(t, err)

			data := make([]byte, (endInd-startInd+1)*util.MaxMerkleLeavesSize)
			fileOffset := FMTSize + nodesSize + startInd*util.MaxMerkleLeavesSize

			_, err = r.Seek(int64(fileOffset), io.SeekStart)
			require.NoError(t, err)

			n, err = r.Read(data)
			require.NoError(t, err)

			data = data[:n]

			vmp := util.MerklePathForMultiLeafVerification{
				RootHash: merkleRoot,
				Index:    indexes,
				Nodes:    nodeHashes,
				DataSize: int64(size),
			}

			err = vmp.VerifyMultipleBlocks(data)
			require.NoError(t, err)
		})

	}

}

func getRandomSize(size int64) int64 {
	for {
		n := rand.Int63n(size)
		if n != 0 {
			return n
		}
	}
}

func getValidationTreeTotalNodes(dataSize int) int {
	totalLeaves := (dataSize + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize
	totalNodes := totalLeaves
	for totalLeaves > 2 {
		totalLeaves = (totalLeaves + 1) / 2
		totalNodes += totalLeaves
	}
	return totalNodes
}

func getRandomIndexRange(dataSize int64) (startInd, endInd int) {
	totalInd := int((dataSize + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize)
	startInd = rand.Intn(totalInd)

	if startInd == totalInd-1 {
		endInd = startInd
		return
	}

	r := totalInd - startInd
	endInd = rand.Intn(r)
	endInd += startInd
	if endInd >= totalInd {
		endInd = totalInd - 1
	}
	return
}
