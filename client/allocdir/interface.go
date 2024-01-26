// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
	tomb "gopkg.in/tomb.v1"
)

func New(alloc *structs.Allocation, logger hclog.Logger, rootDir string) Interface {
	// how to decide what kind of alloc to create?
	return &AllocDir2{
		logger: logger.Named("alloc_dir2"),
	}
}

// AllocDirFS exposes file operations on the alloc dir
type AllocDirFS interface {
	List(path string) ([]*cstructs.AllocFileInfo, error)
	Stat(path string) (*cstructs.AllocFileInfo, error)
	ReadAt(path string, offset int64) (io.ReadCloser, error)
	Snapshot(w io.Writer) error
	BlockUntilExists(ctx context.Context, path string) (chan error, error)
	ChangeEvents(ctx context.Context, path string, curOffset int64) (*watch.FileChanges, error)
}

// Interface is implemented by AllocDir and AllocDir2.
type Interface interface {
	AllocDirFS

	NewTaskDir(string) *TaskDir
	WalkTaskDirs(func(*TaskDir) error) error
	AllocDirPath() string
	ShareDirPath() string
	GetTaskDir(string) *TaskDir
	Build() error
	Destroy() error
	Move(Interface, []*structs.Task) error
}

func list(fs Interface, path string) ([]*cstructs.AllocFileInfo, error) {
	allocDir := fs.AllocDirPath()
	if escapes, err := escapingfs.PathEscapesAllocDir(allocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(allocDir, path)
	finfos, err := os.ReadDir(p)
	if err != nil {
		return []*cstructs.AllocFileInfo{}, err
	}

	files := make([]*cstructs.AllocFileInfo, len(finfos))
	for idx, file := range finfos {
		info, err := file.Info()
		if err != nil {
			return []*cstructs.AllocFileInfo{}, err
		}
		files[idx] = &cstructs.AllocFileInfo{
			Name:     info.Name(),
			IsDir:    info.IsDir(),
			Size:     info.Size(),
			FileMode: info.Mode().String(),
			ModTime:  info.ModTime(),
		}
	}
	return files, err
}

func stat(fs Interface, path string) (*cstructs.AllocFileInfo, error) {
	allocDir := fs.AllocDirPath()
	if escapes, err := escapingfs.PathEscapesAllocDir(allocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(allocDir, path)
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	contentType := detectContentType(info, p)
	return &cstructs.AllocFileInfo{
		Size:        info.Size(),
		Name:        info.Name(),
		IsDir:       info.IsDir(),
		FileMode:    info.Mode().String(),
		ModTime:     info.ModTime(),
		ContentType: contentType,
	}, nil
}

func readAt(fs Interface, path string, offset int64) (io.ReadCloser, error) {
	allocDir := fs.AllocDirPath()
	if escapes, err := escapingfs.PathEscapesAllocDir(allocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(allocDir, path)

	// Check if it is trying to read into a secret directory
	if err := fs.WalkTaskDirs(func(taskDir *TaskDir) error {
		if filepath.HasPrefix(p, taskDir.SecretsDir) {
			return fmt.Errorf("Reading secret file prohibited: %s", path)
		}
		if filepath.HasPrefix(p, taskDir.PrivateDir) {
			return fmt.Errorf("Reading private file prohibited: %s", path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("can't seek to offset %q: %v", offset, err)
	}
	return f, nil
}

func blockUntilExists(fs Interface, ctx context.Context, path string) (chan error, error) {
	allocDir := fs.AllocDirPath()
	if escapes, err := escapingfs.PathEscapesAllocDir(allocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	// Get the path relative to the alloc directory
	p := filepath.Join(allocDir, path)
	watcher := getFileWatcher(p)
	returnCh := make(chan error, 1)
	t := &tomb.Tomb{}
	go func() {
		<-ctx.Done()
		t.Kill(nil)
	}()
	go func() {
		returnCh <- watcher.BlockUntilExists(t)
		close(returnCh)
	}()
	return returnCh, nil
}

func changeEvents(fs Interface, ctx context.Context, path string, curOffset int64) (*watch.FileChanges, error) {
	allocDir := fs.AllocDirPath()
	if escapes, err := escapingfs.PathEscapesAllocDir(allocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	t := &tomb.Tomb{}
	go func() {
		<-ctx.Done()
		t.Kill(nil)
	}()

	// Get the path relative to the alloc directory
	p := filepath.Join(allocDir, path)
	watcher := getFileWatcher(p)
	return watcher.ChangeEvents(t, curOffset)
}

func destroy(fs Interface) error {
	var mErr multierror.Error

	if err := fs.WalkTaskDirs(func(dir *TaskDir) error {
		return unmount(fs, dir)
	}); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	if err := os.RemoveAll(fs.AllocDirPath()); err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to remove alloc dir %q: %v", fs.AllocDirPath(), err))
	}

	return mErr.ErrorOrNil()
}

func unmount(fs Interface, dir *TaskDir) error {
	var mErr multierror.Error

	if pathExists(dir.SharedTaskDir) {
		if err := unlinkDir(dir.SharedTaskDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to unmount shared alloc dir %q: %v", dir.SharedTaskDir, err))
		} else if err := os.RemoveAll(dir.SharedTaskDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to delete shared alloc dir %q: %v", dir.SharedTaskDir, err))
		}
	}

	if pathExists(dir.SecretsDir) {
		if err := removeSecretDir(dir.SecretsDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the secret dir %q: %v", dir.SecretsDir, err))
		}
	}

	if pathExists(dir.PrivateDir) {
		if err := removeSecretDir(dir.PrivateDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the private dir %q: %v", dir.PrivateDir, err))
		}
	}

	// Unmount dev/ and proc/ have been mounted.
	if err := dir.unmountSpecialDirs(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

func snapshot(fs Interface, allocDir, shareDir string, rootPaths []string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	walkFn := func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Include the path of the file name relative to the alloc dir
		// so that we can put the files in the right directories
		relPath, err := filepath.Rel(allocDir, path)
		if err != nil {
			return err
		}
		link := ""
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("error reading symlink: %v", err)
			}
			link = target
		}
		hdr, err := tar.FileInfoHeader(fileInfo, link)
		if err != nil {
			return fmt.Errorf("error creating file header: %v", err)
		}
		hdr.Name = relPath
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		// If it's a directory or symlink we just write the header into the tar
		if fileInfo.IsDir() || (fileInfo.Mode()&os.ModeSymlink != 0) {
			return nil
		}

		// Write the file into the archive
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	}

	// Walk through all the top level directories and add the files and
	// directories in the archive
	for _, path := range rootPaths {
		if err := filepath.Walk(path, walkFn); err != nil {
			allocID := filepath.Base(allocDir)
			if writeErr := writeError(tw, allocID, err); writeErr != nil {
				// This could be bad; other side won't know
				// snapshotting failed. It could also just mean
				// the snapshotting side closed the connect
				// prematurely and won't try to use the tar
				// anyway.
				err = errors.Join(writeErr)
			}
			return fmt.Errorf("failed to snapshot %s: %v", path, err)
		}
	}

	return nil
}
