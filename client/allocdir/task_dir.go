// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper/users/dynamic"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

const (
	// defaultSecretDirTmpfsSize is the default size of the tmpfs per task in MBs
	defaultSecretDirTmpfsSize = 1
)

// TaskDir contains all of the paths relevant to a task. All paths are on the
// host system so drivers should mount/link into task containers as necessary.
type TaskDir struct {
	// AllocDir is the path to the alloc directory on the host.
	// (not to be conflated with client.alloc_dir)
	//
	// <alloc_dir>
	AllocDir string

	// Dir is the path to Task directory on the host.
	//
	// <task_dir>
	Dir string

	// MountsAllocDir is the path to the alloc directory on the host that has
	// been bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/alloc -> <alloc_dir>
	MountsAllocDir string

	// MountsTaskDir is the path to the task directory on the host that has been
	// bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/task -> <task_dir>
	MountsTaskDir string

	// MountsSecretsDir is the path to the secrets directory on the host that
	// has been bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/task/secrets -> <secrets_dir>
	MountsSecretsDir string

	// SharedAllocDir is the path to shared alloc directory on the host
	//
	// <alloc_dir>/alloc/
	SharedAllocDir string

	// SharedTaskDir is the path to the shared alloc directory linked into
	// the task directory on the host.
	//
	// <task_dir>/alloc/
	SharedTaskDir string

	// LocalDir is the path to the task's local directory on the host
	//
	// <task_dir>/local/
	LocalDir string

	// LogDir is the path to the task's log directory on the host
	//
	// <alloc_dir>/alloc/logs/
	LogDir string

	// SecretsDir is the path to secrets/ directory on the host
	//
	// <task_dir>/secrets/
	SecretsDir string

	// secretsInMB is the configured size of the secrets directory
	secretsInMB int

	// PrivateDir is the path to private/ directory on the host
	//
	// <task_dir>/private/
	PrivateDir string

	// skip embedding these paths in chroots. Used for avoiding embedding
	// client.alloc_dir and client.mounts_dir recursively.
	skip *set.Set[string]

	// logger for this task
	logger hclog.Logger
}

// newTaskDir creates a TaskDir struct with paths set. Call Build() to
// create paths on disk.
//
// Call AllocDir.NewTaskDir to create new TaskDirs
func (d *AllocDir) newTaskDir(taskName string, secretsInMB int) *TaskDir {
	taskDir := filepath.Join(d.AllocDir, taskName)
	taskUnique := filepath.Base(d.AllocDir) + "-" + taskName

	if secretsInMB == 0 {
		secretsInMB = defaultSecretDirTmpfsSize
	}

	return &TaskDir{
		AllocDir:         d.AllocDir,
		Dir:              taskDir,
		SharedAllocDir:   filepath.Join(d.AllocDir, SharedAllocName),
		LogDir:           filepath.Join(d.AllocDir, SharedAllocName, LogDirName),
		SharedTaskDir:    filepath.Join(taskDir, SharedAllocName),
		LocalDir:         filepath.Join(taskDir, TaskLocal),
		SecretsDir:       filepath.Join(taskDir, TaskSecrets),
		PrivateDir:       filepath.Join(taskDir, TaskPrivate),
		MountsAllocDir:   filepath.Join(d.clientAllocMountsDir, taskUnique, "alloc"),
		MountsTaskDir:    filepath.Join(d.clientAllocMountsDir, taskUnique),
		MountsSecretsDir: filepath.Join(d.clientAllocMountsDir, taskUnique, "secrets"),
		skip:             set.From[string]([]string{d.clientAllocDir, d.clientAllocMountsDir}),
		logger:           d.logger.Named("task_dir").With("task_name", taskName),
		secretsInMB:      secretsInMB,
	}
}

// Build default directories and permissions in a task directory. chrootCreated
// allows skipping chroot creation if the caller knows it has already been
// done. client.alloc_dir will be skipped.
func (t *TaskDir) Build(fsi fsisolation.Mode, chroot map[string]string, username string) error {
	if err := allocMkdirAll(t.Dir, fileMode777); err != nil {
		return err
	}

	if err := allocMkdirAll(t.LocalDir, fileMode777); err != nil {
		return err
	}

	// Create the directories that should be in every task.
	for dir, perms := range TaskDirs {
		absdir := filepath.Join(t.Dir, dir)

		if err := allocMkdirAll(absdir, perms); err != nil {
			return err
		}
	}

	// Only link alloc dir into task dir for chroot fs isolation.
	// Image based isolation will bind the shared alloc dir in the driver.
	// If there's no isolation the task will use the host path to the
	// shared alloc dir.
	if fsi == fsisolation.Chroot {
		// If the path doesn't exist OR it exists and is empty, link it
		empty, _ := pathEmpty(t.SharedTaskDir)
		if !pathExists(t.SharedTaskDir) || empty {
			if err := linkDir(t.SharedAllocDir, t.SharedTaskDir); err != nil {
				return fmt.Errorf("Failed to mount shared directory for task: %w", err)
			}
		}
	}

	// Create the secret directory
	if err := allocMakeSecretsDir(t.SecretsDir, t.secretsInMB, fileMode777); err != nil {
		return err
	}

	// Create the private directory
	if err := allocMakeSecretsDir(t.PrivateDir, defaultSecretDirTmpfsSize, fileMode777); err != nil {
		return err
	}

	// Build chroot if chroot filesystem isolation is going to be used
	if fsi == fsisolation.Chroot {
		if err := t.buildChroot(chroot); err != nil {
			return err
		}
	}

	// Only bind mount the task alloc/task dirs to the client.mounts_dir/<task>
	if fsi == fsisolation.Unveil {
		uid, gid, _, err := dynamic.LookupUser(username)
		if err != nil {
			return fmt.Errorf("Failed to lookup user: %v", err)
		}

		nobodyUID, nobodyGID, _, err := dynamic.LookupUser("nobody")
		if err != nil {
			return fmt.Errorf("Failed to lookup nobody user: %v", err)
		}

		// create the task unique directory under the client mounts path
		parent := filepath.Dir(t.MountsAllocDir)
		if err = os.MkdirAll(parent, fileMode710); err != nil {
			return fmt.Errorf("Failed to create task mount directory: %v", err)
		}
		if err = os.Chown(parent, uid, gid); err != nil {
			return fmt.Errorf("Failed to chown task mount directory: %v", err)
		}

		// create the taskdir mount point
		if err = mountDir(t.Dir, t.MountsTaskDir, uid, gid, fileMode710); err != nil {
			return fmt.Errorf("Failed to mount task dir: %v", err)
		}

		// create the allocdir mount point (owned by nobody)
		if err = mountDir(filepath.Join(t.AllocDir, "/alloc"), t.MountsAllocDir, nobodyUID, nobodyGID, fileMode777); err != nil {
			return fmt.Errorf("Failed to mount alloc dir: %v", err)
		}

		// create the secretsdir mount point
		if err = mountDir(t.SecretsDir, t.MountsSecretsDir, uid, gid, fileMode710); err != nil {
			return fmt.Errorf("Failed to mount secrets dir: %v", err)
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
		if t.skip.Contains(source) {
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
				return fmt.Errorf("Couldn't create destination directory %v: %w", dest, err)
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
			return fmt.Errorf("Couldn't create destination directory %v: %w", destDir, err)
		}

		// Enumerate the files in source.
		dirEntries, err := os.ReadDir(source)
		if err != nil {
			return fmt.Errorf("Couldn't read directory %v: %w", source, err)
		}

		for _, fileEntry := range dirEntries {
			entry, err := fileEntry.Info()
			if err != nil {
				return fmt.Errorf("Couldn't read the file information %v: %w", entry, err)
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
					return fmt.Errorf("Couldn't resolve symlink for %v: %w", source, err)
				}

				if err := os.Symlink(link, taskEntry); err != nil {
					// Symlinking twice
					if err.(*os.LinkError).Err.Error() != "file exists" {
						return fmt.Errorf("Couldn't create symlink: %w", err)
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

// Unmount or delete task directories. Returns all errors as a multierror.
func (t *TaskDir) Unmount() error {
	mErr := new(multierror.Error)

	// Check if the directory has the shared alloc mounted.
	if pathExists(t.SharedTaskDir) {
		if err := unlinkDir(t.SharedTaskDir); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("failed to unmount shared alloc dir %q: %w", t.SharedTaskDir, err))
		} else if err := os.RemoveAll(t.SharedTaskDir); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("failed to delete shared alloc dir %q: %w", t.SharedTaskDir, err))
		}
	}

	// unmount the alloc mounts alloc dir which is mounted inside the alloc mounts task dir
	if pathExists(t.MountsAllocDir) {
		if err := unlinkDir(t.MountsAllocDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the alloc mounts dir %q: %w", t.MountsAllocDir, err),
			)
		}
	}

	// unmount the alloc mounts task secrets dir which is mounted inside the alloc mounts task dir
	if pathExists(t.MountsSecretsDir) {
		if err := unlinkDir(t.MountsSecretsDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the alloc mounts secrets dir %q: %w", t.MountsSecretsDir, err),
			)
		}
	}

	// unmount the alloc mounts task dir which is a mount of the alloc dir
	if pathExists(t.MountsTaskDir) {
		if err := unlinkDir(t.MountsTaskDir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the alloc mounts task dir %q: %w", t.MountsTaskDir, err),
			)
		}
	}

	// delete the task's parent alloc mounts dir if it exists
	if dir := filepath.Dir(t.MountsAllocDir); pathExists(dir) {
		if err := os.RemoveAll(dir); err != nil {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("failed to remove the task alloc mounts dir %q: %w", dir, err))
		}
	}

	if pathExists(t.SecretsDir) {
		if err := removeSecretDir(t.SecretsDir); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("failed to remove the secret dir %q: %w", t.SecretsDir, err))
		}
	}

	if pathExists(t.PrivateDir) {
		if err := removeSecretDir(t.PrivateDir); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("failed to remove the private dir %q: %w", t.PrivateDir, err))
		}
	}

	// Unmount dev/ and proc/ have been mounted.
	if err := t.unmountSpecialDirs(); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	return mErr.ErrorOrNil()
}
