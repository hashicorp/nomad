// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/helper/subproc"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// githubPrefixSSH is the prefix for downloading via git using ssh from GitHub.
	githubPrefixSSH = "git@github.com:"
)

var ErrSandboxEscape = errors.New("artifact includes symlink that resolves outside of sandbox")

func getURL(taskEnv interfaces.EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
	source := taskEnv.ReplaceEnv(artifact.GetterSource)

	// fixup GitHub SSH URL such as git@github.com:hashicorp/nomad.git
	gitSSH := false
	if strings.HasPrefix(source, githubPrefixSSH) {
		gitSSH = true
		source = source[len(githubPrefixSSH):]
	}

	u, err := url.Parse(source)
	if err != nil {
		return "", &Error{
			URL:         artifact.GetterSource,
			Err:         fmt.Errorf("failed to parse source URL %q: %v", artifact.GetterSource, err),
			Recoverable: false,
		}
	}

	// build the URL by substituting as necessary
	q := u.Query()
	for k, v := range artifact.GetterOptions {
		q.Set(k, taskEnv.ReplaceEnv(v))
	}
	u.RawQuery = q.Encode()

	// add the prefix back if necessary
	sourceURL := u.String()
	if gitSSH {
		sourceURL = fmt.Sprintf("%s%s", githubPrefixSSH, sourceURL)
	}

	return sourceURL, nil
}

func getDestination(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
	destination, escapes := env.ClientPath(artifact.RelativeDest, true)
	if escapes {
		return "", &Error{
			URL:         artifact.GetterSource,
			Err:         fmt.Errorf("artifact destination path escapes alloc directory"),
			Recoverable: false,
		}
	}
	return destination, nil
}

func getMode(artifact *structs.TaskArtifact) getter.ClientMode {
	switch artifact.GetterMode {
	case structs.GetterModeFile:
		return getter.ClientModeFile
	case structs.GetterModeDir:
		return getter.ClientModeDir
	default:
		return getter.ClientModeAny
	}
}

func chownDestination(destination, username string) error {
	if destination == "" || username == "" {
		return nil
	}

	if os.Geteuid() != 0 {
		return nil
	}

	if runtime.GOOS == "windows" {
		return nil
	}

	uid, gid, _, err := users.LookupUnix(username)
	if err != nil {
		return err
	}

	return filepath.Walk(destination, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

func isInsecure(artifact *structs.TaskArtifact) bool {
	return artifact.GetterInsecure
}

func getHeaders(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) map[string][]string {
	m := artifact.GetterHeaders
	if len(m) == 0 {
		return nil
	}
	headers := make(http.Header, len(m))
	for k, v := range m {
		headers.Set(k, env.ReplaceEnv(v))
	}
	return headers
}

// getWritableDirs returns host paths to the task's allocation and task specific
// directories - the locations into which a Task is allowed to download an artifact.
func getWritableDirs(env interfaces.EnvReplacer) (string, string) {
	stub, _ := env.ClientPath("stub", false)
	taskDir := filepath.Dir(stub)
	allocDir := filepath.Dir(taskDir)
	return allocDir, taskDir
}

// environment merges the default minimal environment per-OS with the set of
// environment variables configured to be inherited from the Client
func environment(taskDir string, inherit string) []string {
	chomp := func(s string) []string {
		return strings.FieldsFunc(s, func(c rune) bool {
			return c == ',' || unicode.IsSpace(c)
		})
	}
	env := defaultEnvironment(taskDir)
	for _, name := range chomp(inherit) {
		env[name] = os.Getenv(name)
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result
}

func (s *Sandbox) runCmd(env *parameters) error {
	// find the nomad process
	bin := subproc.Self()

	// If the artifact needs to be inspected, it must first be fetched
	// to a temporary location for inspection, then moved to the configured
	// destination.
	var finalDest string         // original artifact destination
	var at *os.Root              // root on the alloc directory, only set if inspecting
	var atTestRecreateDir string // test path to inform if existing destination directory should be removed
	var atFinalDest string       // final destination path within rooted alloc directory
	var atTemporaryDest string   // temporary destination within rooted alloc directory
	if !env.DisableArtifactInspection && (env.DisableFilesystemIsolation || !lockdownAvailable()) {
		var err error

		finalDest = env.Destination // store path so it can be reset
		if atFinalDest, err = filepath.Rel(env.AllocDir, env.Destination); err != nil {
			return err
		}

		tmpDir, err := os.MkdirTemp(env.AllocDir, "artifact-")
		if err != nil {
			return err
		}

		// Create a new root that is rooted to the alloc directory
		if at, err = os.OpenRoot(env.AllocDir); err != nil {
			return err
		}

		// By default use a directory that does not exist. This
		// is required to prevent certain sources (i.e. git) from
		// erroring when fetching the source.
		env.Destination = filepath.Join(tmpDir, "artifact")
		if atTemporaryDest, err = filepath.Rel(env.AllocDir, env.Destination); err != nil {
			return err
		}

		// Check if the real destination exists. If it does, make
		// the temporary destination exist as well to properly
		// mimic go-getter behavior.
		if st, err := at.Stat(atFinalDest); err == nil {
			// The real destination does exist, so update the temporary
			// location to use the basename of the path so if it is used
			// in error messages it is consistent.
			env.Destination = filepath.Join(tmpDir, filepath.Base(finalDest))
			if atTemporaryDest, err = filepath.Rel(env.AllocDir, env.Destination); err != nil {
				return err
			}

			// Now check if the real destination is a file or directory
			// and create the temporary destination accordingly.
			if st.IsDir() {
				if err := at.Mkdir(atTemporaryDest, 0755); err != nil {
					return err
				}

				// Include a temporary check file within the destination
				// directory. This will be used later to determine if
				// go-getter deleted the destination prior to fetch or
				// if the contents were just merged into the directory.
				f, err := os.CreateTemp(env.Destination, "go-getter-test")
				if err != nil {
					return err
				}
				f.Close()

				if atTestRecreateDir, err = filepath.Rel(env.AllocDir, f.Name()); err != nil {
					return err
				}
			} else {
				f, err := at.OpenFile(atTemporaryDest, os.O_CREATE, 0644)
				if err != nil {
					return err
				}
				f.Close()
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		s.logger.Debug("artifact download destination modified for inspection",
			"temporary", env.Destination, "final", finalDest)

		// before leaving, set the destination back to the
		// original value and cleanup
		defer func() {
			env.Destination = finalDest
			os.RemoveAll(tmpDir)
		}()
	}

	// final method of ensuring subprocess termination
	ctx, cancel := subproc.Context(env.deadline())
	defer cancel()

	// start the subprocess, passing in parameters via stdin
	output := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, bin, SubCommand)
	cmd.Env = environment(env.TaskDir, env.SetEnvironmentVariables)
	cmd.Stdin = env.reader()
	cmd.Stdout = output
	cmd.Stderr = output

	// start & wait for the subprocess to terminate
	if err := cmd.Run(); err != nil {
		msg := subproc.Log(output, s.logger.Error)

		return &Error{
			URL:         env.Source,
			Err:         fmt.Errorf("getter subprocess failed: %v: %v", err, msg),
			Recoverable: true,
		}
	}
	subproc.Log(output, s.logger.Debug)

	// if no root has been defined, no inspection
	// is being performed so return now.
	if at == nil {
		return nil
	}

	// generate the inspector for the destination
	artifactInspector, err := genWalkInspector(env.Destination)
	if err != nil {
		return err
	}

	// inspect the contents to find any unwanted files
	if err := filepath.WalkDir(env.Destination, artifactInspector); err != nil {
		return err
	}

	// if the testRecreateDir value is set, check if the file still
	// exists. if it does, then the destination directory should not
	// be deleted. otherwise, destination path should be removed if
	// it exists.
	pathToRemove := atFinalDest
	if atTestRecreateDir != "" {
		if _, err := at.Stat(atTestRecreateDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		} else if err == nil {
			// the existing destination directory should not be
			// removed but the test file should be removed.
			pathToRemove = atTestRecreateDir
		}
	}

	if err := at.RemoveAll(pathToRemove); err != nil {
		return err
	}

	// if in file mode, simply move the file into place. otherwise
	// merge the directories.
	if env.Mode == getter.ClientModeFile {
		if err := at.MkdirAll(filepath.Dir(atFinalDest), 0755); err != nil {
			return err
		}

		if err := at.Rename(atTemporaryDest, atFinalDest); err != nil {
			return err
		}
	} else {
		// merge the artifact contents into the real destination
		if err := mergeDirectories(at, atTemporaryDest, atFinalDest); err != nil {
			return err
		}
	}

	// the artifact contents will have the owner set correctly
	// but the destination directory will not, so set that now
	// if it was configured
	if env.Chown {
		if err := chownDestination(finalDest, env.User); err != nil {
			return err
		}
	}

	return nil

}

// mergeDirectories will merge the contents of the srcDir into
// the dstDir. This is a destructive action; the contents of
// srcDir are moved into dstDir.
func mergeDirectories(at *os.Root, srcDir, dstDir string) error {
	var entries []fs.DirEntry
	var err error
	if runtime.GOOS == "windows" {
		dirFile, err := at.Open(srcDir)
		if err != nil {
			return err
		}
		defer dirFile.Close()
		entries, err = dirFile.ReadDir(-1)
		if err != nil {
			return err
		}
	} else {
		rd, ok := at.FS().(fs.ReadDirFS)
		if !ok {
			return errors.New("unable to read rooted allocation directory")
		}
		entries, err = rd.ReadDir(srcDir)
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		srcInfo, err := at.Stat(src)
		if err != nil {
			return err
		}

		dstInfo, err := at.Stat(dst)
		if err != nil {
			// if the destination does not exist, the source
			// can be moved directly
			if errors.Is(err, os.ErrNotExist) {
				if err := at.MkdirAll(filepath.Dir(dst), 0755); err != nil {
					return err
				}

				if err := at.Rename(src, dst); err != nil {
					return err
				}

				continue
			}

			return err
		}

		// if both the source and destination are directories
		// merge the source into the destination and proceed
		// to the next entry
		if srcInfo.IsDir() && dstInfo.IsDir() {
			if err := mergeDirectories(at, src, dst); err != nil {
				return err
			}

			continue
		}

		// if both the source and destination are files, a
		// rename is sufficient. otherwise, remove the destination.
		if srcInfo.IsDir() || dstInfo.IsDir() {
			if err := at.RemoveAll(dst); err != nil {
				return err
			}
		}

		if err := at.Rename(src, dst); err != nil {
			return err
		}

	}

	return nil
}

// generateWalkInspector creates a walk function to check for symlinks
// that resolve outside of the rootDir.
func genWalkInspector(rootDir string) (fs.WalkDirFunc, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	var walkFn fs.WalkDirFunc

	walkFn = func(path string, entry fs.DirEntry, err error) error {
		// argument error means an error was encountered reading
		// a directory or getting file info so stop here
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		// Only care about symlinks
		if info.Mode()&fs.ModeSymlink != fs.ModeSymlink {
			return nil
		}

		// Build up the actual path
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}

		toCheck, err := filepath.Abs(resolved)
		if err != nil {
			return err
		}

		// Check that entry is still within sandbox
		isWithin, err := isPathWithin(rootDir, toCheck)
		if err != nil {
			return err
		}

		if !isWithin {
			return ErrSandboxEscape
		}

		return nil
	}
	return walkFn, nil
}

// isPathWithin checks if the toCheckPath is within the rootPath. It
// uses the os.SameFile function to perform the path check so paths
// are compared appropriately based on the filesystem.
func isPathWithin(rootPath, toCheckPath string) (bool, error) {
	rootPath = filepath.Clean(rootPath)
	toCheckPath = filepath.Clean(toCheckPath)

	if len(rootPath) > len(toCheckPath) {
		return false, nil
	}

	rootStat, err := os.Stat(rootPath)
	if err != nil {
		return false, err
	}

	checkStat, err := os.Stat(toCheckPath[0:len(rootPath)])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return os.SameFile(rootStat, checkStat), nil
}
