package filestore

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"

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

	// fmt.Println("L80: leaf at index 1024: ", hex.EncodeToString(p))
	mt := util.MerkleTree{}
	mt.ComputeTree(leaves)

	computedRoot := mt.GetRoot()
	require.Equal(t, computedRoot, hex.EncodeToString(merkleRoot))
}
