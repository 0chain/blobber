package filestore

import (
	"crypto/rand"
	"io"
	mRand "math/rand"
	"os"
	"testing"

	"github.com/0chain/gosdk/core/util"
	"github.com/minio/sha256-simd"
)

func BenchmarkFixedMerkleProofFor10MB(b *testing.B) {
	size := int64(10 * MB)
	filename := "benchmarkFixedMtProofFor10MB"
	runFixedMPBench(b, size, filename)
}

func BenchmarkFixedMerkleProofFor50MB(b *testing.B) {
	size := int64(50 * MB)
	filename := "benchmarkFixedMtProofFor50MB"
	runFixedMPBench(b, size, filename)
}

func BenchmarkFixedMerkleProofFor100MB(b *testing.B) {
	size := int64(100 * MB)
	filename := "benchmarkFixedMtProofFor100MB"
	runFixedMPBench(b, size, filename)
}

func BenchmarkFixedMerkleProofFor500MB(b *testing.B) {
	size := int64(500 * MB)
	filename := "benchmarkFixedMtProofFor500MB"
	runFixedMPBench(b, size, filename)
}

func BenchmarkFixedMerkleProofFor1GB(b *testing.B) {
	size := int64(1024 * MB)
	filename := "benchmarkFixedMtProofFor1GB"
	runFixedMPBench(b, size, filename)
}

func BenchmarkFixedMerkleProofFor10GB(b *testing.B) {
	size := int64(10 * 1024 * MB)
	filename := "benchmarkFixedMtProofFor10GB"
	runFixedMPBench(b, size, filename)
}

func runFixedMPBench(b *testing.B, size int64, filename string) {
	merkleRoot, cleanup, err := getFileAndCleanUpFunc(filename, size)
	if err != nil {
		b.Fatalf("error: %v", err)
	}

	r, err := os.Open(filename)
	if err != nil {
		b.Fatalf("error: %v", err)
	}

	defer cleanup()
	for i := 0; i < b.N; i++ {
		_, err = r.Seek(0, io.SeekStart)
		if err != nil {
			b.Fatalf("error: %v", err)
		}

		idx := mRand.Intn(util.FixedMerkleLeaves)
		fixedMP := NewFMTPRoof(idx, size)
		proof, err := fixedMP.GetMerkleProof(r)
		if err != nil {
			b.Fatalf("error: %v", err)
		}

		_, err = r.Seek(FMTSize, io.SeekStart)
		if err != nil {
			b.Fatalf("error: %v", err)
		}
		proofByte, err := fixedMP.GetLeafContent(r)
		if err != nil {
			b.Fatalf("error: %v", err)
		}

		h := sha256.New()
		_, _ = h.Write(proofByte)

		fp := util.FixedMerklePath{
			LeafHash: h.Sum(nil),
			RootHash: merkleRoot,
			Nodes:    proof,
			LeafInd:  idx,
		}

		if !fp.VerifyMerklePath() {
			b.Fatalf("verify merkle path failed")
		}
	}
}

func getFileAndCleanUpFunc(name string, size int64) ([]byte, func(), error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	b := make([]byte, size)
	if _, err = rand.Read(b); err != nil {
		return nil, nil, err
	}

	fixedMT := getNewFixedMerkleTree()
	_, err = fixedMT.Write(b)
	if err != nil {
		return nil, nil, err
	}
	err = fixedMT.Finalize()
	if err != nil {
		return nil, nil, err
	}

	merkleRoot, err := fixedMT.CalculateRootAndStoreNodes(f)
	if err != nil {
		return nil, nil, err
	}

	_, err = f.Write(b)
	if err != nil {
		return nil, nil, err
	}

	return merkleRoot, func() { os.Remove(name) }, nil
}
