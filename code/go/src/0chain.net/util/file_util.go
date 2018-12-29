package util

import (
	"os"
)

func CreateDirs(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeleteFile(path string) error {
	return os.Remove(path)
}
