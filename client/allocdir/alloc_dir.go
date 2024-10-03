// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
	tomb "gopkg.in/tomb.v1"
)

const (
	// idUnsupported is what the uid/gid will be set to on platforms (eg
	// Windows) that don't support integer ownership identifiers.
	idUnsupported = -1

	// fileMode777 is a constant that represents the file mode rwxrwxrwx
	fileMode777 = os.FileMode(0o777)

	// fileMode710 is a constant that represents the file mode rwx--x---
	fileMode710 = os.FileMode(0o710)

	// fileMode755 is a constant that represents the file mode rwxr-xr-x
	fileMode755 = os.FileMode(0o755)

	// fileMode666 is a constant that represents the file mode rw-rw-rw-
	fileMode666 = os.FileMode(0o666)
)

var (
	// SnapshotErrorTime is the sentinel time that will be used on the
	// error file written by Snapshot when it encounters as error.
	SnapshotErrorTime = time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)

	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// Name of the directory where logs of Tasks are written
	LogDirName = "logs"

	// SharedDataDir is one of the shared allocation directories. It is
	// included in snapshots.
	SharedDataDir = "data"

	// TmpDirName is the name of the temporary directory in each alloc and
	// task.
	TmpDirName = "tmp"

	// The set of directories that exist inside each shared alloc directory.
	SharedAllocDirs = []string{LogDirName, TmpDirName, SharedDataDir}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"

	// TaskSecrets is the name of the secret directory inside each task
	// directory
	TaskSecrets = "secrets"

	// TaskPrivate is the name of the private directory inside each task
	// directory
	TaskPrivate = "private"

	// TaskDirs is the set of directories created in each tasks directory.
	TaskDirs = map[string]os.FileMode{TmpDirName: os.ModeSticky | fileMode777}

	// AllocGRPCSocket is the path relative to the task dir root for the
	// unix socket connected to Consul's gRPC endpoint.
	AllocGRPCSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_grpc.sock")

	// AllocHTTPSocket is the path relative to the task dir root for the unix
	// socket connected to Consul's HTTP endpoint.
	AllocHTTPSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_http.sock")
)

// Interface is implemented by AllocDir.
type Interface interface {
	AllocDirFS

	NewTaskDir(*structs.Task) *TaskDir
	AllocDirPath() string
	ShareDirPath() string
	GetTaskDir(string) *TaskDir
	Build() error
	Destroy() error
	Move(Interface, []*structs.Task) error
}

// AllocDir allows creating, destroying, and accessing an allocation's
// directory. All methods are safe for concurrent use.
type AllocDir struct {
	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string

	// The shared directory is available to all tasks within the same task
	// group.
	SharedDir string

	// TaskDirs is a mapping of task names to their non-shared directory.
	TaskDirs map[string]*TaskDir

	// clientAllocDir is the client agent's root alloc directory. It must
	// be excluded from chroots and is configured via client.alloc_dir.
	clientAllocDir string

	// clientAllocMountsDir is the client agent's mounts directory. It must be
	// excluded from chroots and is configured via client.mounts_dir.
	clientAllocMountsDir string

	// built is true if Build has successfully run
	built bool

	mu sync.RWMutex

	logger hclog.Logger
}

func (a *AllocDir) AllocDirPath() string {
	return a.AllocDir
}

func (a *AllocDir) ShareDirPath() string {
	return a.SharedDir
}

func (a *AllocDir) GetTaskDir(task string) *TaskDir {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.TaskDirs[task]
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

// NewAllocDir initializes the AllocDir struct with allocDir as base path for
// the allocation directory.
func NewAllocDir(logger hclog.Logger, clientAllocDir, clientMountsDir, allocID string) *AllocDir {
	logger = logger.Named("alloc_dir")
	allocDir := filepath.Join(clientAllocDir, allocID)
	shareDir := filepath.Join(allocDir, SharedAllocName)

	return &AllocDir{
		clientAllocDir:       clientAllocDir,
		clientAllocMountsDir: clientMountsDir,
		AllocDir:             allocDir,
		SharedDir:            shareDir,
		TaskDirs:             make(map[string]*TaskDir),
		logger:               logger,
	}
}

// NewTaskDir creates a new TaskDir and adds it to the AllocDirs TaskDirs map.
func (d *AllocDir) NewTaskDir(task *structs.Task) *TaskDir {
	d.mu.Lock()
	defer d.mu.Unlock()

	secretsSize := 0
	if task.Resources != nil {
		secretsSize = task.Resources.SecretsMB
	}

	td := d.newTaskDir(task.Name, secretsSize)
	d.TaskDirs[task.Name] = td
	return td
}

// Snapshot creates an archive of the files and directories in the data dir of
// the allocation and the task local directories
//
// Since a valid tar may have been written even when an error occurs, a special
// file "NOMAD-${ALLOC_ID}-ERROR.log" will be appended to the tar with the
// error message as the contents.
func (d *AllocDir) Snapshot(w io.Writer) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	allocDataDir := filepath.Join(d.SharedDir, SharedDataDir)
	rootPaths := []string{allocDataDir}
	for _, taskdir := range d.TaskDirs {
		rootPaths = append(rootPaths, taskdir.LocalDir)
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	walkFn := func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Include the path of the file name relative to the alloc dir
		// so that we can put the files in the right directories
		relPath, err := filepath.Rel(d.AllocDir, path)
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
			return fmt.Errorf("error creating file header: %w", err)
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
			allocID := filepath.Base(d.AllocDir)
			if writeErr := writeError(tw, allocID, err); writeErr != nil {
				// This could be bad; other side won't know
				// snapshotting failed. It could also just mean
				// the snapshotting side closed the connect
				// prematurely and won't try to use the tar
				// anyway.
				d.logger.Warn("snapshotting failed and unable to write error marker", "error", writeErr)
			}
			return fmt.Errorf("failed to snapshot %s: %w", path, err)
		}
	}

	return nil
}

// Move other alloc directory's shared path and local dir to this alloc dir.
func (d *AllocDir) Move(other Interface, tasks []*structs.Task) error {
	d.mu.RLock()
	if !d.built {
		// Enforce the invariant that Build is called before Move
		d.mu.RUnlock()
		return fmt.Errorf("unable to move to %q - alloc dir is not built", d.AllocDir)
	}

	// Moving is slow and only reads immutable fields, so unlock during heavy IO
	d.mu.RUnlock()

	// Move the data directory
	otherDataDir := filepath.Join(other.ShareDirPath(), SharedDataDir)
	dataDir := filepath.Join(d.SharedDir, SharedDataDir)
	if fileInfo, err := os.Stat(otherDataDir); fileInfo != nil && err == nil {
		os.Remove(dataDir) // remove an empty data dir if it exists
		if err := os.Rename(otherDataDir, dataDir); err != nil {
			return fmt.Errorf("error moving data dir: %w", err)
		}
	}

	// Move the task directories
	for _, task := range tasks {
		otherTaskDir := filepath.Join(other.AllocDirPath(), task.Name)
		otherTaskLocal := filepath.Join(otherTaskDir, TaskLocal)

		fileInfo, err := os.Stat(otherTaskLocal)
		if fileInfo != nil && err == nil {
			// TaskDirs haven't been built yet, so create it
			newTaskDir := filepath.Join(d.AllocDir, task.Name)
			if err := os.MkdirAll(newTaskDir, fileMode777); err != nil {
				return fmt.Errorf("error creating task %q dir: %w", task.Name, err)
			}
			localDir := filepath.Join(newTaskDir, TaskLocal)
			os.Remove(localDir) // remove an empty local dir if it exists
			if err := os.Rename(otherTaskLocal, localDir); err != nil {
				return fmt.Errorf("error moving task %q local dir: %w", task.Name, err)
			}
		}
	}

	return nil
}

// Destroy tears down previously build directory structure.
func (d *AllocDir) Destroy() error {
	// Unmount all mounted shared alloc dirs.
	mErr := new(multierror.Error)
	if err := d.UnmountAll(); err != nil {
		mErr = multierror.Append(mErr, err)
	}

	if err := os.RemoveAll(d.AllocDir); err != nil {
		mErr = multierror.Append(mErr, fmt.Errorf("failed to remove alloc dir %q: %w", d.AllocDir, err))
	}

	// Unset built since the alloc dir has been destroyed.
	d.mu.Lock()
	d.built = false
	d.mu.Unlock()
	return mErr.ErrorOrNil()
}

// UnmountAll linked/mounted directories in task dirs.
func (d *AllocDir) UnmountAll() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	mErr := new(multierror.Error)
	for _, dir := range d.TaskDirs {
		if err := dir.Unmount(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Build the directory tree for an allocation.
func (d *AllocDir) Build() error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.MkdirAll(d.AllocDir, fileMode755); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %w", d.AllocDir, err)
	}

	// Make the shared directory and make it available to all user/groups.
	if err := allocMkdirAll(d.SharedDir, fileMode755); err != nil {
		return err
	}

	// Create shared subdirs
	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := allocMkdirAll(p, fileMode777); err != nil {
			return err
		}
	}

	// Mark as built
	d.mu.Lock()
	d.built = true
	d.mu.Unlock()
	return nil
}

// List returns the list of files at a path relative to the alloc dir
func (d *AllocDir) List(path string) ([]*cstructs.AllocFileInfo, error) {
	if escapes, err := escapingfs.PathEscapesAllocDir(d.AllocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %w", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)
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

// Stat returns information about the file at a path relative to the alloc dir
func (d *AllocDir) Stat(path string) (*cstructs.AllocFileInfo, error) {
	if escapes, err := escapingfs.PathEscapesAllocDir(d.AllocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %w", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)
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

// ReadAt returns a reader for a file at the path relative to the alloc dir
func (d *AllocDir) ReadAt(path string, offset int64) (io.ReadCloser, error) {
	if escapes, err := escapingfs.PathEscapesAllocDir(d.AllocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %w", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)

	// Check if it is trying to read into a secret directory
	d.mu.RLock()
	for _, dir := range d.TaskDirs {
		if caseInsensitiveHasPrefix(p, dir.SecretsDir) {
			d.mu.RUnlock()
			return nil, fmt.Errorf("Reading secret file prohibited: %s", path)
		}
		if caseInsensitiveHasPrefix(p, dir.PrivateDir) {
			d.mu.RUnlock()
			return nil, fmt.Errorf("Reading private file prohibited: %s", path)
		}
	}
	d.mu.RUnlock()

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("can't seek to offset %q: %w", offset, err)
	}
	return f, nil
}

// CaseInsensitiveHasPrefix checks if the prefix is a case-insensitive prefix.
func caseInsensitiveHasPrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

// BlockUntilExists blocks until the passed file relative the allocation
// directory exists. The block can be cancelled with the passed context.
func (d *AllocDir) BlockUntilExists(ctx context.Context, path string) (chan error, error) {
	if escapes, err := escapingfs.PathEscapesAllocDir(d.AllocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %w", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	// Get the path relative to the alloc directory
	p := filepath.Join(d.AllocDir, path)
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

// ChangeEvents watches for changes to the passed path relative to the
// allocation directory. The offset should be the last read offset. The context is
// used to clean up the watch.
func (d *AllocDir) ChangeEvents(ctx context.Context, path string, curOffset int64) (*watch.FileChanges, error) {
	if escapes, err := escapingfs.PathEscapesAllocDir(d.AllocDir, "", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %w", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	t := &tomb.Tomb{}
	go func() {
		<-ctx.Done()
		t.Kill(nil)
	}()

	// Get the path relative to the alloc directory
	p := filepath.Join(d.AllocDir, path)
	watcher := getFileWatcher(p)
	return watcher.ChangeEvents(t, curOffset)
}

// getFileWatcher returns a FileWatcher for the given path.
func getFileWatcher(path string) watch.FileWatcher {
	return watch.NewPollingFileWatcher(path)
}

// fileCopy from src to dst setting the permissions and owner (if uid & gid are
// both greater than 0)
func fileCopy(src, dst string, uid, gid int, perm os.FileMode) error {
	// Do a simple copy.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Couldn't open src file %v: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("Couldn't create destination file %v: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Couldn't copy %q to %q: %w", src, dst, err)
	}

	if uid != idUnsupported && gid != idUnsupported {
		if err := dstFile.Chown(uid, gid); err != nil {
			return fmt.Errorf("Couldn't copy %q to %q: %w", src, dst, err)
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
		mode = fileMode777
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
			mode = fileMode777
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
		Mode:       int64(fileMode666),
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

// allocMkdirAll creates a directory and sets the permissions to the passed
// value. It also sets the owner of the directory to "nobody" on systems that
// allow.
func allocMkdirAll(path string, perms os.FileMode) error {
	// Create the directory
	if err := os.MkdirAll(path, perms); err != nil {
		return err
	}
	// Update the access permissions on the directory
	if err := dropDirPermissions(path, perms); err != nil {
		return err
	}
	return nil
}

// allocMakeSecretsDir creates a directory for sensitive items such as secrets.
// When possible it uses a tmpfs or some other method to prevent it from
// persisting to actual disk.
func allocMakeSecretsDir(path string, size int, perms os.FileMode) error {
	// Create the private directory
	if err := createSecretDir(path, size); err != nil {
		return err
	}
	if err := dropDirPermissions(path, perms); err != nil {
		return err
	}
	return nil
}
