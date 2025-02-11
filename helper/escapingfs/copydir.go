// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package escapingfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyDir copies a directory's contents to a new location, returning an error
// on symlinks. This implementation is roughly the same as the stdlib os.CopyDir
// but with th e important difference that we preserve file modes.
func CopyDir(src, dst string) error {
	srcFs := os.DirFS(src)

	return fs.WalkDir(srcFs, ".", func(oldPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		newPath := filepath.Join(dst, oldPath)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("could not stat directory: %v", err)
			}
			return os.MkdirAll(newPath, info.Mode())
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("copying cannot traverse symlinks")
		}

		r, err := srcFs.Open(oldPath)
		if err != nil {
			return fmt.Errorf("could not open existing file: %v", err)
		}
		defer r.Close()
		info, err := r.Stat()
		if err != nil {
			return fmt.Errorf("could not stat file: %v", err)
		}

		w, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, r); err != nil {
			w.Close()
			return fmt.Errorf("could not copy file: %v", err)
		}
		return w.Close()
	})
}
