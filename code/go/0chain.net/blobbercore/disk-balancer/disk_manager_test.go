package disk_balancer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0chain/gosdk/zmagmacore/crypto"
	"github.com/spf13/viper"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func Test_SelectDisk(t *testing.T) {
	// This test requires 2 disks of approximately the same size.
	// Disk paths must be specified in blobberPart_1 and blobberPart_2.

	setConfig()
	common.SetupRootContext(node.GetNodeContext())
	StartDiskSelectorWorker(common.GetRootContext())

	blobberPart_1 := "/mnt/Blobber_P1/files"
	blobberPart_2 := "/mnt/Blobber_P2/files"
	testFile := "testing"

	tests := [3]struct {
		name       string
		createFile bool
		fileSize   uint64
		wantRoot   string
		wantError  bool
	}{
		{
			name:       "SELECT FIRST PART 1",
			createFile: true,
			fileSize:   2 * 1024 * 1024 * 1024,
			wantRoot:   blobberPart_1,
			wantError:  false,
		},
		{
			name:       "SELECT SECOND PART",
			createFile: true,
			fileSize:   2 * 1024 * 1024 * 1024,
			wantRoot:   blobberPart_2,
			wantError:  false,
		},
		{
			name:       "SELECT FIRST PART 2",
			createFile: false,
			wantRoot:   blobberPart_1,
			wantError:  false,
		},
	}

	for idx := range tests {
		test := tests[idx]

		t.Run(test.name, func(t *testing.T) {
			root, err := GetDiskSelector().GetNextDiskPath()
			if (err != nil) != test.wantError {
				t.Errorf("GetNextDiskPath() error got %v | want %v", err, test.wantError)
			}
			if root != test.wantRoot {
				t.Errorf("GetNextDiskPath() root got %v | want %v", root, test.wantRoot)
			}

			if test.createFile {
				fPath := filepath.Join(test.wantRoot, testFile)
				if err := createFile(fPath, test.fileSize); err != nil {
					t.Fatal("createFile() filed")
				}
			}
		})
	}

	for idx := range tests {
		os.Remove(filepath.Join(tests[idx].wantRoot, testFile))
	}
}

func Test_MoveAllocation(t *testing.T) {
	// This test requires 2 disks of approximately the same size.
	// Disk paths must be specified in blobberPart_1 and blobberPart_2.

	setConfig()
	common.SetupRootContext(node.GetNodeContext())
	StartDiskSelectorWorker(common.GetRootContext())

	blobberPart_1 := "/mnt/Blobber_P1/files"
	blobberPart_2 := "/mnt/Blobber_P2/files"
	testFile := "testing"
	allocationID := crypto.Hash(fmt.Sprint(time.Now().UnixNano()))

	d := &diskTier{}
	dirs := d.generateAllocationPath(blobberPart_1, allocationID)
	fPath := filepath.Join(dirs, testFile)
	_ = os.MkdirAll(dirs, 0777)
	if err := createFile(fPath, 2*1024); err != nil {
		t.Fatal("createFile() filed")
	}

	t.Run("MOVE ALLOCATION", func(t *testing.T) {
		GetDiskSelector().MoveAllocation(blobberPart_1, blobberPart_2, allocationID)
		time.Sleep(1 * time.Second)

		if _, err := os.Stat(dirs); os.IsExist(err) {
			t.Fatalf("MoveAllocation() error got %v", err)
		}
		if _, err := os.Stat(d.generateAllocationPath(blobberPart_2, allocationID)); os.IsNotExist(err) {
			t.Fatalf("MoveAllocation() error got %v", err)
		}
	})

	os.RemoveAll(blobberPart_1)
	os.RemoveAll(blobberPart_2)
}

func Test_GetAvailableDisk(t *testing.T) {
	// This test requires 2 disks of different sizes. (10 GB and 15 GB, for example)
	// Disk paths must be specified in blobberPart_1 and blobberPart_2.
	// The size field must be specified like this:
	// For "SELECT CURRENT ROOT": size <blobberPart_1
	// For "SELECT NOT CURRENT ROOT": size <blobberPart_2 && size> blobberPart_1
	// For "NOT ENOUGH DISK SPASE": size> blobberPart_2

	setConfig()
	common.SetupRootContext(node.GetNodeContext())
	StartDiskSelectorWorker(common.GetRootContext())

	blobberPart_1 := "/mnt/Blobber_P1/files"
	blobberPart_2 := "/mnt/Blobber_P3/files"

	tests := [3]struct {
		name      string
		path      string
		size      int64
		wantPath  string
		wantError bool
	}{
		{
			name:      "DON'T CHANGE CURRENT ROOT",
			path:      blobberPart_1,
			size:      20 * 1024 * 1024 * 1024,
			wantPath:  blobberPart_1,
			wantError: false,
		},
		{
			name:      "MOVE TO ANOTHER ROOT",
			path:      blobberPart_1,
			size:      55 * 1024 * 1024 * 1024,
			wantPath:  blobberPart_2,
			wantError: false,
		},
		{
			name:      "ERR NOT ENOUGH DISK SPASE",
			path:      blobberPart_1,
			size:      155 * 1024 * 1024 * 1024,
			wantPath:  "",
			wantError: true,
		},
	}

	for idx := range tests {
		test := tests[idx]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path, err := GetDiskSelector().GetAvailableDisk(test.path, test.size)
			if (err != nil) != test.wantError {
				t.Errorf("GetAvailableDisk() error got %v | want %v", err, test.wantError)
			}
			if path != test.wantPath {
				t.Errorf("GetAvailableDisk() path got %v | want %v", path, test.wantPath)
			}
		})
	}
}

func Test_diskTier_IsMoves(t *testing.T) {
	// This test requires 1 disk.
	// Disk path must be specified in blobberPart_1.

	setConfig()
	common.SetupRootContext(node.GetNodeContext())
	StartDiskSelectorWorker(common.GetRootContext())

	blobberPart_1 := "/mnt/Blobber_P1/files"
	blobberPart_2 := "/mnt/Blobber_P2/files"
	allocationID := crypto.Hash(fmt.Sprint(time.Now().UnixNano()))

	d := &diskTier{}
	allocPath := d.generateAllocationPath(blobberPart_1, allocationID)
	_ = os.MkdirAll(allocPath, 0777)
	a := &allocationInfo{OldRoot: allocPath, NewRoot: blobberPart_2, ForDelete: true}
	if err := a.prepareAllocation(); err != nil {
		t.Fatalf("prepareAllocation() error %v", err)
	}

	tests := [3]struct {
		name           string
		allocationRoot string
		allocationID   string
		needPath       bool
		isMoves        bool
		wantPath       string
	}{
		{
			name:           "DOES NOT MOVE",
			allocationRoot: blobberPart_1,
			allocationID:   crypto.Hash(fmt.Sprint(time.Now().UnixNano())),
			needPath:       false,
			isMoves:        false,
			wantPath:       blobberPart_1,
		},
		{
			name:           "GOT NEW ROOT PATH",
			allocationRoot: blobberPart_1,
			allocationID:   allocationID,
			needPath:       true,
			isMoves:        true,
			wantPath:       blobberPart_2,
		},
		{
			name:           "NO NEED ROOT PATH",
			allocationRoot: blobberPart_1,
			allocationID:   allocationID,
			needPath:       false,
			isMoves:        true,
			wantPath:       "",
		},
	}

	for idx := range tests {
		test := tests[idx]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			isMoves, path := GetDiskSelector().IsMoves(test.allocationRoot, test.allocationID, test.needPath)
			if isMoves != test.isMoves {
				t.Errorf("IsMoves() isMoves got %v | want %v", isMoves, test.isMoves)
			}
			if path != test.wantPath {
				t.Errorf("IsMoves() path got %v | want %v", path, test.wantPath)
			}
		})
	}
}

func Test_GetCapacity(t *testing.T) {
	// FFor this test, in the wantSize field, you must specify the total free disk space that will be used for user files.
	setConfig()
	common.SetupRootContext(node.GetNodeContext())
	StartDiskSelectorWorker(common.GetRootContext())

	tests := [1]struct {
		name     string
		wantSize int64
	}{
		{
			name:     "OK",
			wantSize: 188955623424,
		},
	}

	for idx := range tests {
		test := tests[idx]

		t.Run(test.name, func(t *testing.T) {
			capacity := GetDiskSelector().GetCapacity()
			if capacity < test.wantSize {
				t.Errorf("GetCapacity() capacity got %v | want %v", capacity, test.wantSize)
			}
		})
	}
}

func createFile(path string, size uint64) error {
	data := make([]byte, int(size)) // Initialize an empty byte slice
	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	defer f.Close()
	_, err = f.Write(data) // Write it to the file
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	return nil
}

func setConfig() {
	config.SetupDefaultConfig()
	// Disk balancer
	config.Configuration.MinDiskSize = viper.GetUint64("min_disk_size")
	config.Configuration.CheckDisksTimeout = viper.GetDuration("check_disk_timeout")
	config.Configuration.MountPoint = viper.GetString("mount_point")
	config.Configuration.Strategy = viper.GetString("strategy")
}
