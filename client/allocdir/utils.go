// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hpcloud/tail/watch"
)

// detectContentType tries to infer the file type by reading the first
// 512 bytes of the file. Json file extensions are special cased.
func detectContentType(fileInfo os.FileInfo, path string) string {
	contentType := "application/octet-stream"
	if !fileInfo.IsDir() {
		f, err := os.Open(path)
		// Best effort content type detection
		// We ignore errors because this is optional information
		if err == nil {
			fileBytes := make([]byte, 512)
			n, err := f.Read(fileBytes)
			if err == nil {
				contentType = http.DetectContentType(fileBytes[:n])
			}
			f.Close()
		}
	}
	// Special case json files
	if strings.HasSuffix(path, ".json") {
		contentType = "application/json"
	}
	return contentType
}

// fileCopy from src to dst setting the permissions and owner (if uid & gid are
// both greater than 0)
func fileCopy(src, dst string, uid, gid int, perm os.FileMode) error {
	// Do a simple copy.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Couldn't open src file %v: %v", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("Couldn't create destination file %v: %v", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Couldn't copy %q to %q: %v", src, dst, err)
	}

	if uid != idUnsupported && gid != idUnsupported {
		if err := dstFile.Chown(uid, gid); err != nil {
			return fmt.Errorf("Couldn't copy %q to %q: %v", src, dst, err)
		}
	}

	return nil
}

// pathExists is a helper function to check if the path exists.
func pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// pathEmpty returns true if a path exists, is listable, and is empty. If the
// path does not exist or is not listable an error is returned.
func pathEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	entries, err := f.Readdir(1)
	if err != nil && err != io.EOF {
		return false, err
	}
	return len(entries) == 0, nil
}

// createDir creates a directory structure inside the basepath. This functions
// preserves the permissions of each of the subdirectories in the relative path
// by looking up the permissions in the host.
func createDir(basePath, relPath string) error {
	filePerms, err := splitPath(relPath)
	if err != nil {
		return err
	}

	// We are going backwards since we create the root of the directory first
	// and then create the entire nested structure.
	for i := len(filePerms) - 1; i >= 0; i-- {
		fi := filePerms[i]
		destDir := filepath.Join(basePath, fi.Name)
		if err := os.MkdirAll(destDir, fi.Perm); err != nil {
			return err
		}

		if fi.Uid != idUnsupported && fi.Gid != idUnsupported {
			if err := os.Chown(destDir, fi.Uid, fi.Gid); err != nil {
				return err
			}
		}
	}
	return nil
}

// fileInfo holds the path and the permissions of a file
type fileInfo struct {
	Name string
	Perm os.FileMode

	// Uid and Gid are unsupported on Windows
	Uid int
	Gid int
}

// splitPath stats each subdirectory of a path. The first element of the array
// is the file passed to this function, and the last element is the root of the
// path.
func splitPath(path string) ([]fileInfo, error) {
	var mode os.FileMode
	fi, err := os.Stat(path)

	// If the path is not present in the host then we respond with the most
	// flexible permission.
	uid, gid := idUnsupported, idUnsupported
	if err != nil {
		mode = os.ModePerm
	} else {
		uid, gid = getOwner(fi)
		mode = fi.Mode()
	}
	var dirs []fileInfo
	dirs = append(dirs, fileInfo{Name: path, Perm: mode, Uid: uid, Gid: gid})
	currentDir := path
	for {
		dir := filepath.Dir(filepath.Clean(currentDir))
		if dir == currentDir {
			break
		}

		// We try to find the permission of the file in the host. If the path is not
		// present in the host then we respond with the most flexible permission.
		uid, gid := idUnsupported, idUnsupported
		fi, err := os.Stat(dir)
		if err != nil {
			mode = os.ModePerm
		} else {
			uid, gid = getOwner(fi)
			mode = fi.Mode()
		}
		dirs = append(dirs, fileInfo{Name: dir, Perm: mode, Uid: uid, Gid: gid})
		currentDir = dir
	}
	return dirs, nil
}

// SnapshotErrorFilename returns the filename which will exist if there was an
// error snapshotting a tar.
func SnapshotErrorFilename(allocID string) string {
	return fmt.Sprintf("NOMAD-%s-ERROR.log", allocID)
}

// writeError writes a special file to a tar archive with the error encountered
// during snapshotting. See Snapshot().
func writeError(tw *tar.Writer, allocID string, err error) error {
	contents := []byte(fmt.Sprintf("Error snapshotting: %v", err))
	hdr := tar.Header{
		Name:       SnapshotErrorFilename(allocID),
		Mode:       0666,
		Size:       int64(len(contents)),
		AccessTime: SnapshotErrorTime,
		ChangeTime: SnapshotErrorTime,
		ModTime:    SnapshotErrorTime,
		Typeflag:   tar.TypeReg,
	}

	if err := tw.WriteHeader(&hdr); err != nil {
		return err
	}

	_, err = tw.Write(contents)
	return err
}

// getFileWatcher returns a FileWatcher for the given path.
func getFileWatcher(path string) watch.FileWatcher {
	return watch.NewPollingFileWatcher(path)
}
