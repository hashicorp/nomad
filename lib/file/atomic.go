// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package file

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-uuid"
)

// WriteAtomicWithPerms creates a temp file with specific permissions and then renames and
// moves it to the path.
func WriteAtomicWithPerms(path string, contents []byte, dirPerms, filePerms os.FileMode) error {

	uuid, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}
	tempPath := fmt.Sprintf("%s-%s.tmp", path, uuid)

	// Make a directory within the current one.
	if err := os.MkdirAll(filepath.Dir(path), dirPerms); err != nil {
		return err
	}

	// File opened with write only permissions. Will be created if it does not exist
	// file is given specific permissions
	fh, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerms)
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempPath) // clean up

	if _, err := fh.Write(contents); err != nil {
		fh.Close()
		return err
	}
	// Commits the current state of the file to disk
	if err := fh.Sync(); err != nil {
		fh.Close()
		return err
	}
	if err := fh.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
