package utils

import (
	"fmt"
	"io"
	"os"
)

func MoveFile(oldpath string, newpath string) error {
	err := os.Rename(oldpath, newpath)
	if err == nil {
		return err
	}
	err = CopyFile(oldpath, newpath)
	if err != nil {
		return err
	}
	return os.Remove(oldpath)
}
func CopyFile(oldpath string, newpath string) error {
	sourceFile, err := os.Open(oldpath)
	if err != nil {
		return fmt.Errorf("src err: %v", err)
	}
	defer sourceFile.Close()
	destinationFile, err := os.Create(newpath)
	if err != nil {
		return fmt.Errorf("dst err: %v", err)
	}
	defer destinationFile.Close()
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("copy err: %v", err)
	}
	return nil
}
