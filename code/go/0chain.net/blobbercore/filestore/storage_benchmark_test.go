package filestore

import (
	"flag"
	"fmt"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	gozap "go.uber.org/zap"
)

var enableDirectIO = flag.Bool("enable_directio", false, "Enable O_DIRECT/direct I/O for WriteFile benchmark")

// Helper to generate a random file of given size with a unique index
func generateRandomFileWithIndex(path string, size int64, idx int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 1024*1024) // 1MB buffer
	var written int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(idx)))
	for written < size-8 {
		n := int64(len(buf))
		if size-8-written < n {
			n = size - 8 - written
		}
		_, _ = rng.Read(buf[:n])
		_, err := f.Write(buf[:n])
		if err != nil {
			return err
		}
		written += n
	}
	// Write the index as the last 8 bytes
	idxBytes := []byte(fmt.Sprintf("%08d", idx))
	_, err = f.Write(idxBytes)
	return err
}

// Minimal config and logger setup for the test
func setupTestConfigAndLogger() {
	// Minimal config
	config.Configuration = config.Config{
		EnableDirectIO: *enableDirectIO,
	}
	logging.Logger, _ = gozap.NewDevelopment() // Or zap.NewNop() for no output
}

func BenchmarkWriteFile_O_DIRECT_Batch(b *testing.B) {
	const (
		fileSize = 4 * 1024 * 1024 // 4MB per file
		nFiles   = 100
	)
	tmpDir := b.TempDir()

	// Generate 1000 random files with unique content (not timed)
	srcFiles := make([]string, nFiles)
	for i := 0; i < nFiles; i++ {
		srcFiles[i] = filepath.Join(tmpDir, fmt.Sprintf("src_%d.data", i))
		if err := generateRandomFileWithIndex(srcFiles[i], fileSize, i); err != nil {
			b.Fatalf("failed to generate file %d: %v", i, err)
		}
	}

	// Prepare FileStore (mock as needed)
	fs := &FileStore{
		mp:      tmpDir,
		mAllocs: make(map[string]*allocation),
		rwMU:    &sync.RWMutex{},
	}

	b.ResetTimer()
	setupTestConfigAndLogger()
	for bench := 0; bench < b.N; bench++ {
		writtenFiles := make([]string, nFiles)
		for i := 0; i < nFiles; i++ {
			src, err := os.Open(srcFiles[i])
			if err != nil {
				b.Fatalf("failed to open src file %d: %v", i, err)
			}
			fileName := fmt.Sprintf("testfile_%d.data", i)
			fileData := &FileInputData{
				Name:         fileName,
				Path:         "/" + fileName,
				FilePathHash: fmt.Sprintf("dummyhash_%d", i),
				Size:         fileSize,
			}
			var infile multipart.File = src

			_, err = fs.WriteFile("allocid", "conid", fileData, infile)
			src.Close()
			if err != nil {
				b.Fatalf("WriteFile failed for file %d: %v", i, err)
			}
			writtenFiles[i] = fs.getTempPathForFile("allocid", fileData.Name, fileData.FilePathHash, "conid")
		}
		// Clean up written files after each batch
		for _, f := range writtenFiles {
			os.Remove(f)
		}
	}
	b.StopTimer()
}
