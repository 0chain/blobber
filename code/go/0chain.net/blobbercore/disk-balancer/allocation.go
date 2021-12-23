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
		Files    []string `json:"files"`
		TempFile string   `json:"filepath"`
		NewRoot  string   `json:"newRoot"`
		OldRoot  string   `json:"oldRoot"`
	}
)

func newAllocationInfo(oldRoot, newRoot string) *allocationInfo {
	return &allocationInfo{
		NewRoot: newRoot,
		OldRoot: oldRoot,
	}
}

func (a *allocationInfo) copyFile(inFile, outFile string) error {
	in, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer in.Close()

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

func (a *allocationInfo) Decode(b []byte) error {
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}

	return nil
}

func (a *allocationInfo) Encode() []byte {
	jsonBody, _ := json.Marshal(a)
	return jsonBody
}

func (a *allocationInfo) getFiles() error {
	err := filepath.Walk(a.OldRoot,
		func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				a.Files = append(a.Files, file)
			}
			return nil
		})

	if err != nil {
		return err
	}

	return nil
}

func (a *allocationInfo) Move(ctx context.Context) error {
	for _, file := range a.Files {
		select {
		case <-ctx.Done():
			return nil
		default:
			newFile := filepath.Join(a.NewRoot, file)
			if err := a.copyFile(file, newFile); err != nil {
				return err
			}
			if err := a.updateInfo(); err != nil {
				return err
			}
			if len(a.Files) == 0 {
				break
			}
		}
	}
}

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

func (a *allocationInfo) updateInfo() error {
	f, _ := os.Open(a.TempFile)
	_, err := f.Write(a.Encode())
	if err != nil {
		return err
	}

	return nil
}
