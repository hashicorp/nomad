// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"fmt"
	"os"
	"path/filepath"

	hclog "github.com/hashicorp/go-hclog"
)

// TaskDir contains all of the paths relevant to a task. All paths are on the
// host system so drivers should mount/link into task containers as necessary.
type TaskDir struct {
	// AllocDir is the path to the alloc directory on the host
	AllocDir string

	// Dir is the path to Task directory on the host
	Dir string

	// SharedAllocDir is the path to shared alloc directory on the host
	// <alloc_dir>/alloc/
	SharedAllocDir string

	// SharedTaskDir is the path to the shared alloc directory linked into
	// the task directory on the host.
	// <task_dir>/alloc/
	SharedTaskDir string

	// LocalDir is the path to the task's local directory on the host
	// <task_dir>/local/
	LocalDir string

	// LogDir is the path to the task's log directory on the host
	// <alloc_dir>/alloc/logs/
	LogDir string

	// SecretsDir is the path to secrets/ directory on the host
	// <task_dir>/secrets/
	SecretsDir string

	// PrivateDir is the path to private/ directory on the host
	// <task_dir>/private/
	PrivateDir string

	// skip embedding these paths in chroots. Used for avoiding embedding
	// client.alloc_dir recursively.
	skip map[string]struct{}

	logger hclog.Logger
}

// newTaskDir creates a TaskDir struct with paths set. Call Build() to
// create paths on disk.
//
// Call AllocDir.NewTaskDir to create new TaskDirs
func newTaskDir(logger hclog.Logger, clientAllocDir, allocDir, taskName string) *TaskDir {
	taskDir := filepath.Join(allocDir, taskName)

	logger = logger.Named("task_dir").With("task_name", taskName)

	// skip embedding client.alloc_dir in chroots
	skip := map[string]struct{}{clientAllocDir: {}}

	return &TaskDir{
		AllocDir:       allocDir,
		Dir:            taskDir,
		SharedAllocDir: filepath.Join(allocDir, SharedAllocName),
		LogDir:         filepath.Join(allocDir, SharedAllocName, LogDirName),
		SharedTaskDir:  filepath.Join(taskDir, SharedAllocName),
		LocalDir:       filepath.Join(taskDir, TaskLocal),
		SecretsDir:     filepath.Join(taskDir, TaskSecrets),
		PrivateDir:     filepath.Join(taskDir, TaskPrivate),
		skip:           skip,
		logger:         logger,
	}
}

// Build default directories and permissions in a task directory. chrootCreated
// allows skipping chroot creation if the caller knows it has already been
// done. client.alloc_dir will be skipped.
func (t *TaskDir) Build(createChroot bool, chroot map[string]string) error {
	if err := os.MkdirAll(t.Dir, 0777); err != nil {
		return err
	}

	// Make the task directory have non-root permissions.
	if err := dropDirPermissions(t.Dir, os.ModePerm); err != nil {
		return err
	}

	// Create a local directory that each task can use.
	if err := os.MkdirAll(t.LocalDir, 0777); err != nil {
		return err
	}

	if err := dropDirPermissions(t.LocalDir, os.ModePerm); err != nil {
		return err
	}

	// Create the directories that should be in every task.
	for dir, perms := range TaskDirs {
		absdir := filepath.Join(t.Dir, dir)
		if err := os.MkdirAll(absdir, perms); err != nil {
			return err
		}

		if err := dropDirPermissions(absdir, perms); err != nil {
			return err
		}
	}

	// Only link alloc dir into task dir for chroot fs isolation.
	// Image based isolation will bind the shared alloc dir in the driver.
	// If there's no isolation the task will use the host path to the
	// shared alloc dir.
	if createChroot {
		// If the path doesn't exist OR it exists and is empty, link it
		empty, _ := pathEmpty(t.SharedTaskDir)
		if !pathExists(t.SharedTaskDir) || empty {
			if err := linkDir(t.SharedAllocDir, t.SharedTaskDir); err != nil {
				return fmt.Errorf("Failed to mount shared directory for task: %v", err)
			}
		}
	}

	// Create the secret directory
	if err := createSecretDir(t.SecretsDir); err != nil {
		return err
	}

	if err := dropDirPermissions(t.SecretsDir, os.ModePerm); err != nil {
		return err
	}

	// Create the private directory
	if err := createSecretDir(t.PrivateDir); err != nil {
		return err
	}

	if err := dropDirPermissions(t.PrivateDir, os.ModePerm); err != nil {
		return err
	}

	// Build chroot if chroot filesystem isolation is going to be used
	if createChroot {
		if err := t.buildChroot(chroot); err != nil {
			return err
		}
	}

	return nil
}

// buildChroot takes a mapping of absolute directory or file paths on the host
// to their intended, relative location within the task directory. This
// attempts hardlink and then defaults to copying. If the path exists on the
// host and can't be embedded an error is returned.
func (t *TaskDir) buildChroot(entries map[string]string) error {
	return t.embedDirs(entries)
}

func (t *TaskDir) embedDirs(entries map[string]string) error {
	subdirs := make(map[string]string)
	for source, dest := range entries {
		if _, ok := t.skip[source]; ok {
			// source in skip list
			continue
		}

		// Check to see if directory exists on host.
		s, err := os.Stat(source)
		if os.IsNotExist(err) {
			continue
		}

		// Embedding a single file
		if !s.IsDir() {
			if err := createDir(t.Dir, filepath.Dir(dest)); err != nil {
				return fmt.Errorf("Couldn't create destination directory %v: %v", dest, err)
			}

			// Copy the file.
			taskEntry := filepath.Join(t.Dir, dest)
			uid, gid := getOwner(s)
			if err := linkOrCopy(source, taskEntry, uid, gid, s.Mode().Perm()); err != nil {
				return err
			}

			continue
		}

		// Create destination directory.
		destDir := filepath.Join(t.Dir, dest)

		if err := createDir(t.Dir, dest); err != nil {
			return fmt.Errorf("Couldn't create destination directory %v: %v", destDir, err)
		}

		// Enumerate the files in source.
		dirEntries, err := os.ReadDir(source)
		if err != nil {
			return fmt.Errorf("Couldn't read directory %v: %v", source, err)
		}

		for _, fileEntry := range dirEntries {
			entry, err := fileEntry.Info()
			if err != nil {
				return fmt.Errorf("Couldn't read the file information %v: %v", entry, err)
			}
			hostEntry := filepath.Join(source, entry.Name())
			taskEntry := filepath.Join(destDir, filepath.Base(hostEntry))
			if entry.IsDir() {
				subdirs[hostEntry] = filepath.Join(dest, filepath.Base(hostEntry))
				continue
			}

			// Check if entry exists. This can happen if restarting a failed
			// task.
			if _, err := os.Lstat(taskEntry); err == nil {
				continue
			}

			if !entry.Mode().IsRegular() {
				// If it is a symlink we can create it, otherwise we skip it.
				if entry.Mode()&os.ModeSymlink == 0 {
					continue
				}

				link, err := os.Readlink(hostEntry)
				if err != nil {
					return fmt.Errorf("Couldn't resolve symlink for %v: %v", source, err)
				}

				if err := os.Symlink(link, taskEntry); err != nil {
					// Symlinking twice
					if err.(*os.LinkError).Err.Error() != "file exists" {
						return fmt.Errorf("Couldn't create symlink: %v", err)
					}
				}
				continue
			}

			uid, gid := getOwner(entry)
			if err := linkOrCopy(hostEntry, taskEntry, uid, gid, entry.Mode().Perm()); err != nil {
				return err
			}
		}
	}

	// Recurse on self to copy subdirectories.
	if len(subdirs) != 0 {
		return t.embedDirs(subdirs)
	}

	return nil
}
