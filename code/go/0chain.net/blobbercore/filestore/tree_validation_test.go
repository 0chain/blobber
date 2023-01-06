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
