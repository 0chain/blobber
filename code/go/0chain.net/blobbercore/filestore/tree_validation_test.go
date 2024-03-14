package filestore

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
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
	var size int64
	for i := 0; i < 100; i++ {
		if i < 50 {
			size = getRandomSize(50 * MB)
		} else {
			size = getRandomSize(64 * KB)
		}

		t.Run(fmt.Sprintf("Write test with size %d", size), func(t *testing.T) {
			b := make([]byte, size)
			n, err := rand.Read(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			t.Log("testing with data-size: ", n)

			ft := getNewFixedMerkleTree()
			n, err = ft.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)
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

		})

	}
}

func randomInt(max int64) int64 {
	i := big.NewInt(max)
	randInt, err := rand.Int(rand.Reader, i)
	if err != nil {
		return 0
	}
	return randInt.Int64()
}

func TestFixedMerkleTreeProof(t *testing.T) {
	var size int64
	var index int
	for i := 0; i < 100; i++ {
		if i < 50 {
			size = getRandomSize(50 * MB)
		} else {
			size = getRandomSize(64 * KB)
		}

		index = int(randomInt(1024))

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

			n, err = f.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			merkleRoot, err := ft.CalculateRootAndStoreNodes(f)
			require.NoError(t, err)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)

			finfo, err := r.Stat()
			require.NoError(t, err)
			require.EqualValues(t, size+FMTSize, finfo.Size())

			fm := fixedMerkleTreeProof{
				idx:      index,
				dataSize: int64(size),
				offset:   int64(size),
			}

			proof, err := fm.GetMerkleProof(r)
			require.NoError(t, err)
			fileReader := io.LimitReader(r, size)
			proofByte, err := fm.GetLeafContent(fileReader)
			require.NoError(t, err)

			leafHash := encryption.ShaHash(proofByte)
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
		var size int64
		if i < 50 {
			size = getRandomSize(50 * MB)
		} else {
			size = getRandomSize(64 * KB)
		}

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

			n, err = f.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			merkleRoot1, err := vt.CalculateRootAndStoreNodes(f, size)
			require.NoError(t, err)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)

			totalLeaves := int((size + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize)
			leaves := make([][]byte, totalLeaves)
			if totalLeaves == 1 {
				b := make([]byte, size)
				n, err = r.Read(b)
				require.NoError(t, err)
				require.EqualValues(t, size, n)
				leaves[0] = encryption.ShaHash(b)
			} else {
				_, err = r.Seek(size, io.SeekStart)
				require.NoError(t, err)
				nodes := make([]byte, totalLeaves*HashSize)
				n, err = r.Read(nodes)
				require.NoError(t, err)
				require.EqualValues(t, len(nodes), n)

				for i := 0; i < totalLeaves; i++ {
					off := i * HashSize
					leaves[i] = nodes[off : off+HashSize]
				}
			}

			t.Log("Length of leaves: ", len(leaves))
			t.Log("0th leaf: ", hex.EncodeToString(leaves[0]))

			v := util.ValidationTree{}
			v.SetLeaves(leaves)
			merkleRoot2 := v.GetValidationRoot()
			require.True(t, bytes.Equal(merkleRoot, merkleRoot1))
			require.Equal(t, hex.EncodeToString(merkleRoot), hex.EncodeToString(merkleRoot2))
		})

	}
}

func TestValidationMerkleProof(t *testing.T) {
	for i := 0; i < 100; i++ {
		var size int64
		if i < 50 {
			size = getRandomSize(50 * MB)
		} else {
			size = getRandomSize(64 * KB)
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

			n, err = f.Write(b)
			require.NoError(t, err)
			require.EqualValues(t, size, n)

			fixedMerkleBytes := make([]byte, FMTSize)
			_, err = f.Write(fixedMerkleBytes)
			require.NoError(t, err)

			merkleRoot, err := vt.CalculateRootAndStoreNodes(f, size)
			require.NoError(t, err)

			f.Close()

			r, err := os.Open(filename)
			require.NoError(t, err)

			finfo, err := r.Stat()
			require.NoError(t, err)

			nodesSize := getNodesSize(size, util.MaxMerkleLeavesSize)

			expectedSize := FMTSize + size + nodesSize
			require.EqualValues(t, expectedSize, finfo.Size(), fmt.Sprint("Diff is: ", finfo.Size()-int64(expectedSize)))

			vp := validationTreeProof{
				totalLeaves: int((size + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize),
				dataSize:    int64(size),
				offset:      int64(size) + FMTSize,
			}

			t.Logf("StartInd: %d; endInd: %d", startInd, endInd)
			nodeHashes, indexes, err := vp.GetMerkleProofOfMultipleIndexes(r, nodesSize, startInd, endInd)
			require.NoError(t, err)

			data := make([]byte, (endInd-startInd+1)*util.MaxMerkleLeavesSize)
			fileOffset := int64(startInd * util.MaxMerkleLeavesSize)

			_, err = r.Seek(int64(fileOffset), io.SeekStart)
			require.NoError(t, err)
			fileReader := io.LimitReader(r, size-fileOffset)

			n, err = fileReader.Read(data)
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
		n := randomInt(size)
		if n != 0 {
			return n
		}
	}
}

func getRandomIndexRange(dataSize int64) (startInd, endInd int) {
	totalInd := int((dataSize + util.MaxMerkleLeavesSize - 1) / util.MaxMerkleLeavesSize)
	startInd = int(randomInt(int64(totalInd)))

	if startInd == totalInd-1 {
		endInd = startInd
		return
	}

	r := totalInd - startInd
	endInd = int(randomInt(int64(r)))
	endInd += startInd
	if endInd >= totalInd {
		endInd = totalInd - 1
	}
	return
}
