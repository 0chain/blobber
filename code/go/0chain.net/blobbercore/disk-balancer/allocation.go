package disk_balancer

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type (
	allocationInfo struct {
		Files     []string
		TempFile  string
		NewRoot   string `json:"newRoot"`
		OldRoot   string
		ForDelete bool `json:"forDelete"`
	}
)

func newAllocationInfo(oldRoot, newRoot string) *allocationInfo {
	return &allocationInfo{
		NewRoot: newRoot,
		OldRoot: oldRoot,
	}
}

// copyFile copies the file to the specified directory. If the directory does not exist, it will create it.
func (a *allocationInfo) copyFile(inFile, outFile string) error {
	in, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer in.Close()

	dir, _ := filepath.Split(outFile)
	if err = a.createDirs(dir); err != nil {
		return err
	}

	out, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// createDirs creates all directories in the specified path if they do not exist.
func (a *allocationInfo) createDirs(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

// Decode performs Unmarshal data.
func (a *allocationInfo) Decode(b []byte) error {
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}

	return nil
}

// Encode performs Marshal data.
func (a *allocationInfo) Encode() []byte {
	jsonBody, _ := json.Marshal(a)
	return jsonBody
}

// getFiles generates a list of the contents of the specified directory using filepath.Walk.
func (a *allocationInfo) getFiles() error {
	return filepath.Walk(a.OldRoot,
		func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			a.Files = append(a.Files, file)
			return nil
		})
}

// Move copies the files of the prepared allocation.
func (a *allocationInfo) Move(ctx context.Context) error {
	for _, file := range a.Files {
		select {
		case <-ctx.Done():
			return nil
		default:
			newFile := filepath.Join(a.NewRoot, file)
			info, _ := os.Stat(file)
			if info.IsDir() {
				a.createDirs(newFile)
				continue
			}
			oldFile := filepath.Join(a.OldRoot, file)
			if err := a.copyFile(oldFile, newFile); err != nil {
				return err
			}
			if len(a.Files) == 0 {
				a.ForDelete = true
				a.updateInfo()
				break
			}
		}
	}

	return nil
}

// PrepareAllocation collects data and prepares allocation for relocation.
func (a allocationInfo) PrepareAllocation() error {
	if err := a.getFiles(); err != nil {
		return err
	}
	fPath := filepath.Join(a.OldRoot, TempAllocationFile)
	f, _ := os.Create(fPath)
	defer f.Close()
	a.TempFile = fPath

	_, err := f.Write(a.Encode())
	if err != nil {
		return err
	}

	return nil
}

// updateInfo updates the allocation information file.
func (a *allocationInfo) updateInfo() error {
	f, _ := os.Open(a.TempFile)
	_, err := f.Write(a.Encode())
	if err != nil {
		return err
	}

	return nil
}
