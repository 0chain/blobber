package disk_balancer

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// decode performs Unmarshal data.
func (a *allocationInfo) decode(b []byte) error {
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}

	return nil
}

// encode performs Marshal data.
func (a *allocationInfo) encode() []byte {
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

// move copies the files of the prepared allocation.
func (a *allocationInfo) move(ctx context.Context) error {
	for _, file := range a.Files {
		select {
		case <-ctx.Done():
			return nil
		default:
			reg := regexp.MustCompile(UserFiles)
			loc := reg.FindStringIndex(file)
			rezFile := file[loc[1]:]
			newFile := filepath.Join(a.NewRoot, rezFile)
			info, _ := os.Stat(file)
			if info.IsDir() {
				a.createDirs(newFile)
				continue
			}
			if err := a.copyFile(file, newFile); err != nil {
				return err
			}
		}
	}

	a.ForDelete = true
	a.updateInfo()

	return nil
}

// prepareAllocation collects data and prepares allocation for relocation.
func (a *allocationInfo) prepareAllocation() error {
	if err := a.getFiles(); err != nil {
		return err
	}
	fPath := filepath.Join(a.OldRoot, TempAllocationFile)
	f, _ := os.Create(fPath)
	defer f.Close()
	a.TempFile = fPath

	_, err := f.Write(a.encode())
	if err != nil {
		return err
	}

	return nil
}

// updateInfo updates the allocation information file.
func (a *allocationInfo) updateInfo() error {
	f, _ := os.Create(a.TempFile)
	defer f.Close()
	_, err := f.Write(a.encode())
	if err != nil {
		return err
	}

	return nil
}
