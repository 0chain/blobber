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

var minFileSize = flag.Float64("min_file_size", 1.0, "Minimum file size in MB (can be fractional, e.g., 0.01 for 10KB)")
var maxFileSize = flag.Float64("max_file_size", 10.0, "Maximum file size in MB (can be fractional)")
var nFiles = flag.Int("n_files", 5000, "Number of files to generate")

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

// generateRandomFilesInRange generates n files in dir with random sizes between minMB and maxMB (in MB, float64),
// each file has a unique index in its name and content. Returns the list of file paths and their sizes.
func generateRandomFilesInRange(dir string, n int, minMB, maxMB float64) ([]string, []int64, error) {
	if minMB > maxMB || minMB <= 0 || n <= 0 {
		return nil, nil, fmt.Errorf("invalid input parameters")
	}
	files := make([]string, n)
	sizes := make([]int64, n)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < n; i++ {
		sizeMB := minMB + rng.Float64()*(maxMB-minMB)
		size := int64(sizeMB * 1024 * 1024)
		if size < 1 {
			size = 1 // at least 1 byte
		}
		files[i] = filepath.Join(dir, fmt.Sprintf("src_%d.data", i))
		sizes[i] = size
		if err := generateRandomFileWithIndex(files[i], size, i); err != nil {
			return nil, nil, fmt.Errorf("failed to generate file %d: %w", i, err)
		}
	}
	return files, sizes, nil
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
	tmpDir := b.TempDir()

	// Generate files of random size given on the range of min and max file size
	srcFiles, _, err := generateRandomFilesInRange(tmpDir, *nFiles, *minFileSize, *maxFileSize)
	if err != nil {
		b.Fatalf("failed to generate files: %v", err)
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
		writtenFiles := make([]string, *nFiles)
		for i := 0; i < *nFiles; i++ {
			src, err := os.Open(srcFiles[i])
			if err != nil {
				b.Fatalf("failed to open src file %d: %v", i, err)
			}
			fileName := fmt.Sprintf("testfile_%d.data", i)
			fileData := &FileInputData{
				Name:         fileName,
				Path:         "/" + fileName,
				FilePathHash: fmt.Sprintf("dummyhash_%d", i),
				// Size:         fileSize,
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
